package main

import (
	"fmt"
	"strings"

	"snav/internal/candidate"
	"snav/internal/highlighter"

	"github.com/charmbracelet/lipgloss"
)

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
	text := "up/down move  pgup/pgdn jump  tab preview  ctrl+space copy  enter open file  esc quit"
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
	start := max(m.offset, 0)
	end := min(len(m.filtered), start+rows)

	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		cand, ok := m.candidateForFiltered(i)
		if !ok {
			continue
		}
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

func (m model) candidateForFiltered(i int) (candidate.Candidate, bool) {
	if i < 0 || i >= len(m.filtered) {
		return candidate.Candidate{}, false
	}

	filtered := m.filtered[i]
	idx := int(filtered.Index)
	if idx < 0 || idx >= len(m.candidates) {
		return candidate.Candidate{}, false
	}

	cand := m.candidates[idx]
	if filtered.OpenLine > 0 {
		cand.Line = int(filtered.OpenLine)
		if filtered.OpenCol > 0 {
			cand.Col = int(filtered.OpenCol)
		} else {
			cand.Col = 1
		}
	}

	return cand, true
}

func (m model) renderCandidateLines(cand candidate.Candidate, selected bool, width int) (string, string) {
	lineA := renderLocationLine(cand.File, cand.Line, cand.Col, width, selected, m.queryRunes)

	text := truncateText(cand.Text, width)
	req := m.highlightRequest(cand.LangID, cand.File, cand.Line, text)
	spans := m.lookupHighlightSpans(req, text)

	lineB := renderTokenLine(text, spans, selected, m.queryRunes)
	lineA = padRightANSI(lineA, width)
	lineB = padRightANSI(lineB, width)

	return lineA, lineB
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
	maxCode := max(0, width-7)
	for i := 0; i < avail && i < len(m.preview.Lines); i++ {
		lineNo := m.preview.StartLine + i
		prefix := fmt.Sprintf("%6d ", lineNo)
		prefixRendered := numStyle.Render(prefix)

		selected := lineNo == m.preview.SelectedLine
		text := truncateText(m.preview.Lines[i], maxCode)
		req := m.highlightRequest(m.preview.Lang, m.preview.File, lineNo, text)
		spans := m.lookupHighlightSpans(req, text)
		code := renderTokenLine(text, spans, selected, nil)
		lines = append(lines, prefixRendered+padRightANSI(code, maxCode))
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return lipgloss.NewStyle().Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m model) selectedCandidate() (candidate.Candidate, bool) {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return candidate.Candidate{}, false
	}
	return m.candidateForFiltered(m.cursor)
}

func (m model) highlightRequest(lang highlighter.LangID, file string, line int, text string) highlighter.HighlightRequest {
	req := highlighter.HighlightRequest{
		Lang: lang,
		Text: text,
		Mode: m.cfg.HighlightMode,
	}
	if m.cfg.HighlightMode == highlighter.HighlightContextFile {
		req.File = file
		req.Line = line
	}
	return req
}

func (m model) lookupHighlightSpans(req highlighter.HighlightRequest, text string) []highlighter.Span {
	spans, ok := m.highlighter.Lookup(req)
	if ok {
		return spans
	}
	m.highlighter.Queue(req)
	return []highlighter.Span{{Start: 0, End: utf8RuneCount(text), Cat: highlighter.TokenPlain}}
}

func (m model) rowsPerPage() int {
	_, listH, _, _ := m.layout()
	return m.rowsPerPageWithHeight(listH)
}

func (m model) rowsPerPageWithHeight(h int) int {
	return max(1, h/2)
}

func (m model) layout() (listWidth int, listHeight int, previewWidth int, previewHeight int) {
	headerHeight := 2
	footerHeight := 1
	contentH := max(m.height-headerHeight-footerHeight, 1)

	if !m.previewEnabled || m.width < 90 {
		return m.width, contentH, 0, 0
	}

	previewWidth = max(30, (m.width*9+10)/20)
	listWidth = m.width - previewWidth - 1
	if listWidth < 20 {
		listWidth = m.width
		previewWidth = 0
	}
	return listWidth, contentH, previewWidth, contentH
}
