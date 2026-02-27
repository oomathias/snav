package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"snav/internal/candidate"
	"snav/internal/highlighter"
	"snav/internal/readfile"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type config struct {
	Root          string
	Pattern       string
	Preview       bool
	CacheSize     int
	Workers       int
	Debounce      time.Duration
	VisibleBuffer int
	HighlightMode highlighter.HighlightContextMode
	ContextRadius int
	EditorCmd     string
	NoIgnore      bool
	ExcludeTests  bool
	Theme         string
}

type previewState struct {
	File         string
	Lang         highlighter.LangID
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
	queryRaw   []rune
	queryRunes []rune

	candidates []candidate.Candidate
	filtered   []candidate.FilteredCandidate

	cursor int
	offset int

	producerOut     <-chan candidate.Candidate
	producerDone    <-chan error
	scanDone        bool
	producerCfg     candidate.ProducerConfig
	rebuildFromScan bool
	scanCandidates  []candidate.Candidate

	highlighter *highlighter.Highlighter

	filterPending          bool
	filterDue              time.Time
	resetSelectionOnFilter bool
	lastFilterQueryRunes   []rune
	lastFilterCandidateN   int

	previewEnabled bool
	preview        previewState
	fileCache      map[string][]string
	fileLangCache  map[string]highlighter.LangID
	previewKey     string

	status string
	errMsg string
}

type tickMsg struct{}

func printUsageWithLongFlags(fs *flag.FlagSet, program string) {
	out := fs.Output()
	fmt.Fprintf(out, "Usage of %s:\n", program)

	var b strings.Builder
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(out)

	text := b.String()
	if text == "" {
		return
	}

	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if strings.HasPrefix(line, "  -") {
			line = "  --" + strings.TrimPrefix(line, "  -")
		}
		fmt.Fprintln(out, line)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func newModel(cfg config, out <-chan candidate.Candidate, done <-chan error, hl *highlighter.Highlighter) model {
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
		queryRaw:       nil,
		queryRunes:     nil,
		producerOut:    out,
		producerDone:   done,
		highlighter:    hl,
		previewEnabled: cfg.Preview,
		fileCache:      make(map[string][]string),
		fileLangCache:  make(map[string]highlighter.LangID),
	}
}

func (m *model) useCachedIndex(candidates []candidate.Candidate) {
	if len(candidates) == 0 {
		return
	}

	m.rebuildFromScan = true
	m.scanCandidates = make([]candidate.Candidate, 0, len(candidates))
	m.candidates = candidates
	m.filtered = make([]candidate.FilteredCandidate, len(candidates))
	for i := range candidates {
		m.filtered[i] = candidate.FilteredCandidate{Index: int32(i)}
	}
	m.lastFilterCandidateN = len(candidates)
	m.status = fmt.Sprintf("using cached index (%d symbols)", len(candidates))
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
		returnAfterPreview := func() (tea.Model, tea.Cmd) {
			m.updatePreview()
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k", "ctrl+p":
			m.moveCursor(-1)
			return returnAfterPreview()
		case "down", "j", "ctrl+n":
			m.moveCursor(1)
			return returnAfterPreview()
		case "pgup", "ctrl+u":
			m.moveCursor(-m.rowsPerPage())
			return returnAfterPreview()
		case "pgdown", "ctrl+d":
			m.moveCursor(m.rowsPerPage())
			return returnAfterPreview()
		case "home":
			m.cursor = 0
			return returnAfterPreview()
		case "end":
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return returnAfterPreview()
		case "tab":
			m.previewEnabled = !m.previewEnabled
			m.previewKey = ""
			return returnAfterPreview()
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
		case "ctrl+@", "ctrl+space":
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
			m.queryRaw = candidate.TrimRunes(next)
			m.queryRunes = candidate.LowerRunes(m.queryRaw)
			m.resetSelectionOnFilter = true
			m.scheduleFilter(m.cfg.Debounce)
		}
		return m, cmd
	}

	return m, nil
}

func (m *model) moveCursor(delta int) {
	m.cursor += delta
	m.ensureCursor()
}

func (m *model) resetSelection() {
	m.cursor = 0
	m.offset = 0
}

func (m *model) ensureCursor() {
	if len(m.filtered) == 0 {
		m.resetSelection()
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}

	page := m.rowsPerPage()
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
	needFilter := false
	defer func() {
		if needFilter && !m.resetSelectionOnFilter {
			m.scheduleFilter(0)
		}
	}()

	for range maxItems {
		select {
		case cand, ok := <-m.producerOut:
			if !ok {
				m.producerOut = nil
				return
			}
			if m.rebuildFromScan {
				m.scanCandidates = append(m.scanCandidates, cand)
				continue
			}
			m.candidates = append(m.candidates, cand)
			if len(m.queryRunes) == 0 {
				m.filtered = append(m.filtered, candidate.FilteredCandidate{Index: int32(len(m.candidates) - 1)})
			} else {
				needFilter = true
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
		m.scanDone = true
		m.producerDone = nil
		if ok && err != nil {
			m.errMsg = err.Error()
			return
		}

		if m.rebuildFromScan {
			m.rebuildFromScan = false
			m.candidates = m.scanCandidates
			m.scanCandidates = nil
			m.filtered = nil
			m.lastFilterCandidateN = 0
			m.lastFilterQueryRunes = nil
			m.resetSelectionOnFilter = true
			m.scheduleFilter(0)
			m.status = fmt.Sprintf("index refreshed (%d symbols)", len(m.candidates))
		}

		cacheCfg := m.producerCfg
		cacheCandidates := m.candidates
		go func() {
			_ = SaveIndexCache(cacheCfg, cacheCandidates)
		}()
	default:
	}
}

func (m *model) scheduleFilter(delay time.Duration) {
	m.filterPending = true
	m.filterDue = time.Now().Add(delay)
}

func (m *model) applyFilter() {
	m.filterPending = false
	sameQuery := slices.Equal(m.queryRunes, m.lastFilterQueryRunes)
	if len(m.candidates) == m.lastFilterCandidateN && sameQuery {
		return
	}

	resetSelection := m.resetSelectionOnFilter
	m.resetSelectionOnFilter = false

	selectedID := 0
	if !resetSelection {
		if cand, ok := m.selectedCandidate(); ok {
			selectedID = cand.ID
		}
	}

	candidateN := len(m.candidates)
	if !resetSelection && sameQuery && len(m.queryRunes) > 0 && candidateN > m.lastFilterCandidateN {
		added := candidate.FilterCandidatesRangeWithQueryRunes(m.candidates, m.lastFilterCandidateN, candidateN, m.queryRaw, m.queryRunes)
		m.filtered = candidate.MergeFilteredCandidates(m.candidates, m.filtered, added)
	} else if shouldUseIncrementalFilter(m.queryRunes, m.lastFilterQueryRunes, candidateN, m.lastFilterCandidateN) {
		m.filtered = candidate.FilterCandidatesSubsetWithQueryRunes(m.candidates, m.filtered, m.queryRaw, m.queryRunes)
	} else {
		m.filtered = candidate.FilterCandidatesWithQueryRunes(m.candidates, m.queryRaw, m.queryRunes)
	}
	m.lastFilterQueryRunes = copyRunesReuse(m.lastFilterQueryRunes, m.queryRunes)
	m.lastFilterCandidateN = candidateN

	if len(m.filtered) == 0 {
		m.previewKey = ""
	}
	if len(m.filtered) == 0 || resetSelection || selectedID == 0 {
		m.resetSelection()
		return
	}

	for i := range m.filtered {
		cand := m.candidates[int(m.filtered[i].Index)]
		if cand.ID == selectedID {
			m.cursor = i
			m.ensureCursor()
			return
		}
	}

	m.resetSelection()
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
		cand, ok := m.candidateForFiltered(i)
		if !ok {
			continue
		}
		text := truncateText(cand.Text, listW)
		m.highlighter.Queue(m.highlightRequest(cand.LangID, cand.File, cand.Line, text))
	}

	if !m.previewEnabled || len(m.preview.Lines) == 0 || previewH <= 1 {
		return
	}

	lines := m.preview.Lines
	visible := min(len(lines), previewH-1)
	maxCode := max(0, previewW-7)
	for i := range visible {
		lineNo := m.preview.StartLine + i
		text := truncateText(lines[i], maxCode)
		m.highlighter.Queue(m.highlightRequest(m.preview.Lang, m.preview.File, lineNo, text))
	}
}

func (m *model) updatePreview() {
	cand, ok := m.selectedCandidate()
	if !m.previewEnabled || !ok {
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
		lang = highlighter.DetectLanguageWithShebang(cand.File, fileLines[0])
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
	lines, err := readfile.ReadLinesNormalized(abs)
	if err != nil {
		return nil, err
	}
	m.fileCache[rel] = lines
	return lines, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	var cfg config
	flag.StringVar(&cfg.Root, "root", ".", "search root")
	flag.StringVar(&cfg.Pattern, "pattern", candidate.DefaultRGPattern, "ripgrep regex pattern")
	flag.BoolVar(&cfg.Preview, "preview", true, "show preview pane")
	flag.IntVar(&cfg.CacheSize, "cache-size", 20000, "highlight cache entries")
	flag.IntVar(&cfg.Workers, "workers", max(1, runtime.GOMAXPROCS(0)-1), "highlight workers")
	flag.IntVar(&cfg.VisibleBuffer, "visible-buffer", 30, "extra rows to pre-highlight")
	flag.IntVar(&cfg.ContextRadius, "context-radius", 40, "line radius for file context highlighting")
	flag.StringVar(&cfg.EditorCmd, "editor-cmd", "", "override open command, supports {file} {line} {col} {target}")
	flag.BoolVar(&cfg.NoIgnore, "no-ignore", false, "disable rg ignore files (.gitignore/.ignore/.rgignore)")
	flag.BoolVar(&cfg.ExcludeTests, "exclude-tests", false, "exclude common test directories and test filename patterns")
	flag.StringVar(&cfg.Theme, "theme", "nord", "color theme (for example: nord, dracula, monokai, github, solarized-dark)")
	highlightContext := flag.String("highlight-context", string(highlighter.HighlightContextFile), "highlight mode: synthetic or file")
	debounceMs := flag.Int("debounce-ms", 100, "query debounce in milliseconds")
	flag.Usage = func() {
		printUsageWithLongFlags(flag.CommandLine, os.Args[0])
	}
	flag.Parse()
	cfg.Debounce = time.Duration(*debounceMs) * time.Millisecond

	if err := SetTheme(cfg.Theme); err != nil {
		fatalf("invalid --theme: %v", err)
	}

	mode, err := highlighter.ParseHighlightContextMode(*highlightContext)
	if err != nil {
		fatalf("invalid --highlight-context: %v", err)
	}
	cfg.HighlightMode = mode

	if flag.NArg() > 0 {
		cfg.Root = flag.Arg(0)
	}

	absRoot, err := filepath.Abs(cfg.Root)
	if err != nil {
		fatalf("resolve root: %v", err)
	}
	cfg.Root = absRoot

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producerPattern := strings.TrimSpace(cfg.Pattern)
	if producerPattern == "" {
		producerPattern = candidate.DefaultRGPattern
	}

	producerCfg := candidate.ProducerConfig{
		Root:         cfg.Root,
		Pattern:      producerPattern,
		NoIgnore:     cfg.NoIgnore,
		ExcludeTests: cfg.ExcludeTests,
	}

	cachedCandidates, cacheLoaded, cacheErr := LoadIndexCache(producerCfg)
	out, done := candidate.StartProducer(ctx, producerCfg)

	highlighter := highlighter.NewHighlighter(highlighter.HighlighterConfig{
		CacheSize:     cfg.CacheSize,
		Workers:       cfg.Workers,
		Root:          cfg.Root,
		DefaultMode:   cfg.HighlightMode,
		ContextRadius: cfg.ContextRadius,
	})
	m := newModel(cfg, out, done, highlighter)
	m.producerCfg = producerCfg
	if cacheErr != nil {
		m.status = "index cache unavailable: " + cacheErr.Error()
	}
	if cacheLoaded {
		m.useCachedIndex(cachedCandidates)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fatalf("snav failed: %v", err)
	}
}
