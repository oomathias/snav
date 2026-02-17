package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

type config struct {
	Root          string
	Pattern       string
	Preview       bool
	CacheSize     int
	Workers       int
	Debounce      time.Duration
	VisibleBuffer int
	HighlightMode HighlightContextMode
	ContextRadius int
	EditorCmd     string
	NoIgnore      bool
	NoTest        bool
	Theme         string
}

type previewState struct {
	File         string
	Lang         LangID
	StartLine    int
	Lines        []string
	SelectedLine int
	Err          string
}

type model struct {
	cfg config

	width  int
	height int

	input      textinput.Model
	query      string
	queryRunes []rune

	candidates []Candidate
	filtered   []filteredCandidate

	cursor int
	offset int

	producerOut  <-chan Candidate
	producerDone <-chan error
	scanDone     bool

	highlighter *Highlighter

	filterPending bool
	filterDue     time.Time

	previewEnabled bool
	preview        previewState
	fileCache      map[string][]string
	fileLangCache  map[string]LangID
	previewKey     string

	status string
	errMsg string
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func newModel(cfg config, out <-chan Candidate, done <-chan error, highlighter *Highlighter) model {
	input := textinput.New()
	input.Prompt = "query> "
	input.Focus()
	input.CharLimit = 256
	input.SetValue("")
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Accent))

	return model{
		cfg:            cfg,
		input:          input,
		query:          "",
		queryRunes:     nil,
		producerOut:    out,
		producerDone:   done,
		highlighter:    highlighter,
		previewEnabled: cfg.Preview,
		fileCache:      make(map[string][]string),
		fileLangCache:  make(map[string]LangID),
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(16, m.width-16)
		m.scheduleFilter(0)

	case tickMsg:
		m.drainProducer(4000)
		m.drainProducerDone()

		if m.filterPending && time.Now().After(m.filterDue) {
			m.applyFilter()
		}

		m.ensureCursor()
		m.updatePreview()
		m.queueVisibleHighlights()

		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k", "ctrl+p":
			m.moveCursor(-1)
			m.updatePreview()
			return m, nil
		case "down", "j", "ctrl+n":
			m.moveCursor(1)
			m.updatePreview()
			return m, nil
		case "pgup", "ctrl+u":
			m.moveCursor(-m.rowsPerPage())
			m.updatePreview()
			return m, nil
		case "pgdown", "ctrl+d":
			m.moveCursor(m.rowsPerPage())
			m.updatePreview()
			return m, nil
		case "home":
			m.cursor = 0
			m.updatePreview()
			return m, nil
		case "end":
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			m.updatePreview()
			return m, nil
		case "tab":
			m.previewEnabled = !m.previewEnabled
			m.previewKey = ""
			m.updatePreview()
			return m, nil
		case "enter":
			cand, ok := m.selectedCandidate()
			if !ok {
				return m, nil
			}
			abs := filepath.Join(m.cfg.Root, cand.File)
			if err := openLocation(abs, cand.Line, cand.Col, m.cfg.EditorCmd); err != nil {
				m.status = "open failed: " + err.Error()
				return m, nil
			}
			return m, tea.Quit
		case "y":
			cand, ok := m.selectedCandidate()
			if !ok {
				return m, nil
			}
			loc := fmt.Sprintf("%s:%d:%d", cand.File, cand.Line, cand.Col)
			if err := copyToClipboard(loc); err != nil {
				m.status = "copy failed: " + err.Error()
			} else {
				m.status = "copied " + loc
			}
			return m, nil
		}

		prev := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		next := m.input.Value()
		if next != prev {
			m.query = next
			m.queryRunes = lowerTrimRunes(next)
			m.scheduleFilter(m.cfg.Debounce)
		}
		return m, cmd
	}

	return m, nil
}

func (m *model) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.ensureCursor()
}

func (m *model) ensureCursor() {
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}

	page := m.rowsPerPage()
	if page < 1 {
		page = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+page {
		m.offset = m.cursor - page + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	maxOffset := max(0, len(m.filtered)-page)
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

func (m *model) drainProducer(maxItems int) {
	for i := 0; i < maxItems; i++ {
		select {
		case cand, ok := <-m.producerOut:
			if !ok {
				m.producerOut = nil
				return
			}
			m.candidates = append(m.candidates, cand)
			if strings.TrimSpace(m.query) == "" {
				m.filtered = append(m.filtered, filteredCandidate{Index: len(m.candidates) - 1})
			} else {
				m.scheduleFilter(m.cfg.Debounce)
			}
		default:
			return
		}
	}
}

func (m *model) drainProducerDone() {
	if m.scanDone || m.producerDone == nil {
		return
	}
	select {
	case err, ok := <-m.producerDone:
		if !ok {
			m.scanDone = true
			m.producerDone = nil
			return
		}
		m.scanDone = true
		m.producerDone = nil
		if err != nil {
			m.errMsg = err.Error()
		}
	default:
	}
}

func (m *model) scheduleFilter(delay time.Duration) {
	m.filterPending = true
	m.filterDue = time.Now().Add(delay)
}

func (m *model) applyFilter() {
	m.filterPending = false
	selectedID := 0
	if cand, ok := m.selectedCandidate(); ok {
		selectedID = cand.ID
	}

	m.filtered = filterCandidates(m.candidates, m.query)
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.offset = 0
		m.previewKey = ""
		return
	}

	if selectedID == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}

	for i := range m.filtered {
		cand := m.candidates[m.filtered[i].Index]
		if cand.ID == selectedID {
			m.cursor = i
			m.ensureCursor()
			return
		}
	}

	m.cursor = 0
	m.offset = 0
}

func (m *model) queueVisibleHighlights() {
	if m.highlighter == nil {
		return
	}

	listW, listH, previewW, previewH := m.layout()
	if listW <= 0 || listH <= 0 {
		return
	}

	start := max(0, m.offset-m.cfg.VisibleBuffer)
	end := min(len(m.filtered), m.offset+m.rowsPerPage()+m.cfg.VisibleBuffer)
	for i := start; i < end; i++ {
		cand := m.candidates[m.filtered[i].Index]
		text := truncateText(cand.Text, listW)
		m.highlighter.Queue(m.highlightRequestForCandidate(cand, text))
	}

	if !m.previewEnabled || len(m.preview.Lines) == 0 {
		return
	}

	if previewH <= 1 {
		return
	}

	lines := m.preview.Lines
	visible := min(len(lines), previewH-1)
	maxCode := max(0, previewW-7)
	for i := 0; i < visible; i++ {
		lineNo := m.preview.StartLine + i
		text := truncateText(lines[i], maxCode)
		m.highlighter.Queue(m.highlightRequestForPreview(m.preview.Lang, m.preview.File, lineNo, text))
	}
}

func (m *model) updatePreview() {
	if !m.previewEnabled {
		m.preview = previewState{}
		m.previewKey = ""
		return
	}

	cand, ok := m.selectedCandidate()
	if !ok {
		m.preview = previewState{}
		m.previewKey = ""
		return
	}

	key := fmt.Sprintf("%s:%d:%d", cand.File, cand.Line, m.height)
	if key == m.previewKey {
		return
	}
	m.previewKey = key

	fileLines, err := m.loadFile(cand.File)
	if err != nil {
		m.preview = previewState{File: cand.File, Err: err.Error()}
		return
	}
	if len(fileLines) == 0 {
		m.preview = previewState{File: cand.File, Err: "empty file"}
		return
	}

	lang := m.fileLangCache[cand.File]
	if lang == "" {
		first := ""
		if len(fileLines) > 0 {
			first = fileLines[0]
		}
		lang = DetectLanguageWithShebang(cand.File, first)
		m.fileLangCache[cand.File] = lang
	}

	_, _, _, previewH := m.layout()
	visible := max(1, previewH-1)
	before := visible / 4
	start := max(1, cand.Line-before)
	end := min(len(fileLines), start+visible-1)
	if end-start+1 < visible {
		start = max(1, end-visible+1)
	}

	lines := fileLines[start-1 : end]
	m.preview = previewState{
		File:         cand.File,
		Lang:         lang,
		StartLine:    start,
		Lines:        lines,
		SelectedLine: cand.Line,
	}
}

func (m *model) loadFile(rel string) ([]string, error) {
	if lines, ok := m.fileCache[rel]; ok {
		return lines, nil
	}
	abs := filepath.Join(m.cfg.Root, rel)
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	m.fileCache[rel] = lines
	return lines, nil
}

func (m model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	header := m.renderHeader()
	listW, listH, previewW, previewH := m.layout()

	listView := m.renderList(listW, listH)
	main := listView
	if m.previewEnabled && previewW > 0 {
		previewView := m.renderPreview(previewW, previewH)
		main = lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", previewView)
	}

	footer := m.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

func (m model) renderHeader() string {
	queryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Text)).Background(lipgloss.Color(appTheme.InputBG)).Padding(0, 1)
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Muted))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Error))

	scanState := "scanning"
	if m.scanDone {
		scanState = "done"
	}
	status := fmt.Sprintf("%s | candidates %d | visible %d", scanState, len(m.candidates), len(m.filtered))
	if m.status != "" {
		status += " | " + m.status
	}

	line1 := queryStyle.Render(m.input.View())
	line2 := statusStyle.Render(status)
	if m.errMsg != "" {
		line2 += "  " + errStyle.Render(m.errMsg)
	}
	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

func (m model) renderFooter() string {
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Muted))
	text := "up/down move  pgup/pgdn jump  tab preview  y copy  enter open file  esc quit"
	return footerStyle.Render(truncateText(text, m.width))
}

func (m model) renderList(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	if len(m.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Muted)).Width(width).Height(height)
		return emptyStyle.Render("no matches")
	}

	rows := m.rowsPerPageWithHeight(height)
	if rows <= 0 {
		rows = 1
	}
	start := m.offset
	if start < 0 {
		start = 0
	}
	end := min(len(m.filtered), start+rows)

	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		cand := m.candidates[m.filtered[i].Index]
		lineA, lineB := m.renderCandidateLines(cand, i == m.cursor, width)
		lines = append(lines, lineA)
		if len(lines) < height {
			lines = append(lines, lineB)
		}
		if len(lines) >= height {
			break
		}
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m model) renderCandidateLines(cand Candidate, selected bool, width int) (string, string) {
	lineA := renderLocationLine(cand.File, cand.Line, cand.Col, width, selected, m.queryRunes)

	text := truncateText(cand.Text, width)
	req := m.highlightRequestForCandidate(cand, text)
	spans, ok := m.highlighter.Lookup(req)
	if !ok {
		m.highlighter.Queue(req)
		spans = []Span{{Start: 0, End: utf8RuneCount(text), Cat: TokenPlain}}
	}

	lineB := renderTokenLine(text, spans, selected, m.queryRunes)
	lineA = padRightANSI(lineA, width)
	lineB = padRightANSI(lineB, width)

	return lineA, lineB
}

func renderLocationLine(path string, line int, col int, width int, selected bool, queryRunes []rune) string {
	loc, fileStart, fileEnd := formatLocationWithVisibleFilename(path, line, col, width)
	runes := []rune(loc)
	if len(runes) == 0 {
		return ""
	}

	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.PathDir))
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.PathFile))
	suffixStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.PathMeta))
	if selected {
		dirStyle = dirStyle.Background(lipgloss.Color(appTheme.SelectionBG))
		fileStyle = fileStyle.Background(lipgloss.Color(appTheme.SelectionBG))
		suffixStyle = suffixStyle.Background(lipgloss.Color(appTheme.SelectionBG))
	}

	emphasis := buildEmphasisMask(len(runes), fuzzyPositionsRunes(loc, queryRunes))

	partAt := func(i int) int {
		if i < fileStart {
			return 0
		}
		if i < fileEnd {
			return 1
		}
		return 2
	}

	styleAt := func(i int) lipgloss.Style {
		switch partAt(i) {
		case 0:
			return dirStyle
		case 1:
			return fileStyle
		default:
			return suffixStyle
		}
	}

	var b strings.Builder
	for i := 0; i < len(runes); {
		baseStyle := styleAt(i)
		part := partAt(i)
		emph := emphasisAt(emphasis, i)
		j := i + 1
		for j < len(runes) {
			if emphasisAt(emphasis, j) != emph {
				break
			}
			if partAt(j) != part {
				break
			}
			j++
		}
		style := baseStyle
		if emph {
			style = style.Bold(true).Underline(true)
		}
		b.WriteString(style.Render(string(runes[i:j])))
		i = j
	}

	return padRightANSI(b.String(), width)
}

func formatLocationWithVisibleFilename(path string, line int, col int, width int) (string, int, int) {
	suffix := fmt.Sprintf(":%d:%d", line, col)
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	if dir == "." {
		dir = ""
	}
	if dir != "" && !strings.HasSuffix(dir, "/") {
		dir += "/"
	}

	baseSuffix := base + suffix
	baseSuffixW := lipgloss.Width(baseSuffix)
	if width <= 0 {
		return "", 0, 0
	}

	if baseSuffixW >= width {
		tr := truncateText(baseSuffix, width)
		fileEnd := utf8RuneCount(tr)
		fileLen := utf8RuneCount(base)
		if fileLen > fileEnd {
			fileLen = fileEnd
		}
		return tr, 0, fileLen
	}

	availDir := width - baseSuffixW
	dirVisible := dir
	if lipgloss.Width(dirVisible) > availDir {
		dirVisible = truncateText(dirVisible, availDir)
	}

	loc := dirVisible + baseSuffix
	loc = truncateText(loc, width)
	fileStart := utf8RuneCount(dirVisible)
	fileEnd := fileStart + utf8RuneCount(base)
	locLen := utf8RuneCount(loc)
	if fileStart > locLen {
		fileStart = locLen
	}
	if fileEnd > locLen {
		fileEnd = locLen
	}
	return loc, fileStart, fileEnd
}

func (m model) renderPreview(width int, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Header)).Bold(true)
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Dim))

	if m.preview.Err != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Error))
		msg := headerStyle.Render("preview") + "\n" + errStyle.Render(truncateText(m.preview.Err, width))
		return lipgloss.NewStyle().Width(width).Height(height).Render(msg)
	}
	if len(m.preview.Lines) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	lines := make([]string, 0, height)
	lines = append(lines, headerStyle.Render(truncateText("preview  "+m.preview.File, width)))

	avail := height - 1
	if avail < 0 {
		avail = 0
	}
	for i := 0; i < avail && i < len(m.preview.Lines); i++ {
		lineNo := m.preview.StartLine + i
		prefix := fmt.Sprintf("%6d ", lineNo)
		prefixRendered := numStyle.Render(prefix)

		selected := lineNo == m.preview.SelectedLine
		maxCode := max(0, width-7)
		text := truncateText(m.preview.Lines[i], maxCode)
		req := m.highlightRequestForPreview(m.preview.Lang, m.preview.File, lineNo, text)
		spans, ok := m.highlighter.Lookup(req)
		if !ok {
			m.highlighter.Queue(req)
			spans = []Span{{Start: 0, End: utf8RuneCount(text), Cat: TokenPlain}}
		}
		code := renderTokenLine(text, spans, selected, m.queryRunes)
		lines = append(lines, prefixRendered+padRightANSI(code, maxCode))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m model) selectedCandidate() (Candidate, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return Candidate{}, false
	}
	return m.candidates[m.filtered[m.cursor].Index], true
}

func (m model) highlightRequestForCandidate(cand Candidate, text string) HighlightRequest {
	req := HighlightRequest{
		Lang: cand.LangID,
		Text: text,
		Mode: m.cfg.HighlightMode,
	}
	if m.cfg.HighlightMode == HighlightContextFile {
		req.File = cand.File
		req.Line = cand.Line
	}
	return req
}

func (m model) highlightRequestForPreview(lang LangID, file string, line int, text string) HighlightRequest {
	req := HighlightRequest{
		Lang: lang,
		Text: text,
		Mode: m.cfg.HighlightMode,
	}
	if m.cfg.HighlightMode == HighlightContextFile {
		req.File = file
		req.Line = line
	}
	return req
}

func (m model) rowsPerPage() int {
	_, listH, _, _ := m.layout()
	return m.rowsPerPageWithHeight(listH)
}

func (m model) rowsPerPageWithHeight(h int) int {
	if h <= 0 {
		return 1
	}
	rows := h / 2
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m model) layout() (listWidth int, listHeight int, previewWidth int, previewHeight int) {
	headerHeight := 2
	footerHeight := 1
	contentH := m.height - headerHeight - footerHeight
	if contentH < 1 {
		contentH = 1
	}

	if !m.previewEnabled || m.width < 90 {
		return m.width, contentH, 0, 0
	}

	previewWidth = max(30, m.width/3)
	listWidth = m.width - previewWidth - 1
	if listWidth < 20 {
		listWidth = m.width
		previewWidth = 0
	}
	return listWidth, contentH, previewWidth, contentH
}

func renderTokenLine(text string, spans []Span, selected bool, queryRunes []rune) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	if len(spans) == 0 {
		spans = []Span{{Start: 0, End: len(runes), Cat: TokenPlain}}
	}

	emphasis := buildEmphasisMask(len(runes), fuzzyPositionsRunes(text, queryRunes))

	var b strings.Builder
	for _, span := range spans {
		start := clamp(span.Start, 0, len(runes))
		end := clamp(span.End, 0, len(runes))
		if end <= start {
			continue
		}
		for i := start; i < end; {
			emph := emphasisAt(emphasis, i)
			j := i + 1
			for j < end && emphasisAt(emphasis, j) == emph {
				j++
			}
			style := tokenStyle(span.Cat, selected)
			if emph {
				style = style.Bold(true).Underline(true)
			}
			b.WriteString(style.Render(string(runes[i:j])))
			i = j
		}
	}

	return b.String()
}

func tokenStyle(cat TokenCategory, selected bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Text))
	if selected {
		style = style.Background(lipgloss.Color(appTheme.SelectionBG))
	}

	switch cat {
	case TokenKeyword:
		return style.Foreground(lipgloss.Color(appTheme.Keyword))
	case TokenType:
		return style.Foreground(lipgloss.Color(appTheme.Type))
	case TokenFunction:
		return style.Foreground(lipgloss.Color(appTheme.Function))
	case TokenString:
		return style.Foreground(lipgloss.Color(appTheme.String))
	case TokenNumber:
		return style.Foreground(lipgloss.Color(appTheme.Number))
	case TokenComment:
		return style.Foreground(lipgloss.Color(appTheme.Comment))
	case TokenOperator:
		return style.Foreground(lipgloss.Color(appTheme.Operator)).Faint(true)
	case TokenError:
		return style.Foreground(lipgloss.Color(appTheme.Error)).Bold(true)
	default:
		return style.Foreground(lipgloss.Color(appTheme.Text))
	}
}

func openLocation(path string, line int, col int, editorCmd string) error {
	target := fmt.Sprintf("%s:%d:%d", path, line, col)

	if strings.TrimSpace(editorCmd) != "" {
		name, args, err := buildEditorCommand(editorCmd, path, line, col, target)
		if err != nil {
			return err
		}
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("editor command not found: %s", name)
		}
		return exec.Command(name, args...).Start()
	}

	if _, err := exec.LookPath("zed"); err == nil {
		cmd := exec.Command("zed", target)
		return cmd.Start()
	}

	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("open"); err == nil {
			return exec.Command("open", path).Start()
		}
		return fmt.Errorf("zed and open are unavailable")
	case "linux":
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", path).Start()
		}
		return fmt.Errorf("zed and xdg-open are unavailable")
	case "windows":
		if _, err := exec.LookPath("explorer.exe"); err == nil {
			return exec.Command("explorer.exe", path).Start()
		}
		if _, err := exec.LookPath("cmd"); err == nil {
			return exec.Command("cmd", "/C", "start", "", path).Start()
		}
		return fmt.Errorf("zed and explorer are unavailable")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func buildEditorCommand(template string, file string, line int, col int, target string) (string, []string, error) {
	parts := strings.Fields(strings.TrimSpace(template))
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("editor command is empty")
	}

	repl := map[string]string{
		"{file}":   file,
		"{line}":   fmt.Sprintf("%d", line),
		"{col}":    fmt.Sprintf("%d", col),
		"{target}": target,
	}

	for i := range parts {
		for k, v := range repl {
			parts[i] = strings.ReplaceAll(parts[i], k, v)
		}
	}

	return parts[0], parts[1:], nil
}

func copyToClipboard(s string) error {
	switch runtime.GOOS {
	case "darwin":
		return pipeStringToCommand(s, "pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return pipeStringToCommand(s, "wl-copy")
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			return pipeStringToCommand(s, "xclip", "-selection", "clipboard")
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return pipeStringToCommand(s, "xsel", "--clipboard", "--input")
		}
		return fmt.Errorf("no clipboard utility found (install wl-copy, xclip, or xsel)")
	case "windows":
		return pipeStringToCommand(s, "clip")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func pipeStringToCommand(input string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(in, input); err != nil {
		_ = in.Close()
		_ = cmd.Wait()
		return err
	}
	if err := in.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func truncateText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", "    ")

	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return runewidth.Truncate(s, maxWidth, "")
	}
	return runewidth.Truncate(s, maxWidth, "...")
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}

func padRightANSI(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func clamp(v int, lo int, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildEmphasisMask(runeLen int, positions []int) []bool {
	if runeLen <= 0 || len(positions) == 0 {
		return nil
	}
	mask := make([]bool, runeLen)
	for _, pos := range positions {
		if pos >= 0 && pos < runeLen {
			mask[pos] = true
		}
	}
	return mask
}

func emphasisAt(mask []bool, idx int) bool {
	if idx < 0 || idx >= len(mask) {
		return false
	}
	return mask[idx]
}

func main() {
	var cfg config
	flag.StringVar(&cfg.Root, "root", ".", "search root")
	flag.StringVar(&cfg.Pattern, "pattern", defaultRGPattern, "ripgrep regex pattern")
	flag.BoolVar(&cfg.Preview, "preview", true, "show preview pane")
	flag.IntVar(&cfg.CacheSize, "cache-size", 20000, "highlight cache entries")
	flag.IntVar(&cfg.Workers, "workers", max(1, runtime.GOMAXPROCS(0)-1), "highlight workers")
	flag.IntVar(&cfg.VisibleBuffer, "visible-buffer", 30, "extra rows to pre-highlight")
	flag.IntVar(&cfg.ContextRadius, "context-radius", 40, "line radius for file context highlighting")
	flag.StringVar(&cfg.EditorCmd, "editor-cmd", "", "override open command, supports {file} {line} {col} {target}")
	flag.BoolVar(&cfg.NoIgnore, "no-ignore", false, "disable rg ignore files (.gitignore/.ignore/.rgignore)")
	flag.BoolVar(&cfg.NoTest, "no-test", false, "exclude common test directories and test filename patterns")
	flag.StringVar(&cfg.Theme, "theme", "nord", "color theme (for example: nord, dracula, monokai, github, solarized-dark)")
	highlightContext := flag.String("highlight-context", string(HighlightContextSynthetic), "highlight mode: synthetic or file")
	debounceMs := flag.Int("debounce-ms", 10, "query debounce in milliseconds")
	flag.Parse()
	cfg.Debounce = time.Duration(*debounceMs) * time.Millisecond

	if err := SetTheme(cfg.Theme); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -theme: %v\n", err)
		os.Exit(1)
	}

	mode, err := ParseHighlightContextMode(*highlightContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -highlight-context: %v\n", err)
		os.Exit(1)
	}
	cfg.HighlightMode = mode

	if flag.NArg() > 0 {
		cfg.Root = flag.Arg(0)
	}

	absRoot, err := filepath.Abs(cfg.Root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		os.Exit(1)
	}
	cfg.Root = absRoot

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, done := StartProducer(ctx, ProducerConfig{
		Root:     cfg.Root,
		Pattern:  cfg.Pattern,
		NoIgnore: cfg.NoIgnore,
		NoTest:   cfg.NoTest,
	})

	highlighter := NewHighlighter(HighlighterConfig{
		CacheSize:     cfg.CacheSize,
		Workers:       cfg.Workers,
		Root:          cfg.Root,
		DefaultMode:   cfg.HighlightMode,
		ContextRadius: cfg.ContextRadius,
	})
	m := newModel(cfg, out, done, highlighter)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "snav failed: %v\n", err)
		os.Exit(1)
	}
}
