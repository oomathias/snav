package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"snav/internal/candidate"
	"snav/internal/highlighter"

	"github.com/charmbracelet/lipgloss"
)

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

	emphasis := buildEmphasisMask(len(runes), candidate.FuzzyPositionsRunes(loc, queryRunes))

	partAt := func(i int) int {
		if i < fileStart {
			return 0
		}
		if i < fileEnd {
			return 1
		}
		return 2
	}

	var b strings.Builder
	for i := 0; i < len(runes); {
		part := partAt(i)
		baseStyle := suffixStyle
		switch part {
		case 0:
			baseStyle = dirStyle
		case 1:
			baseStyle = fileStyle
		}
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

	return b.String()
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
		fileLen := min(utf8RuneCount(base), fileEnd)
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

func renderTokenLine(text string, spans []highlighter.Span, selected bool, queryRunes []rune) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	if len(spans) == 0 {
		spans = []highlighter.Span{{Start: 0, End: len(runes), Cat: highlighter.TokenPlain}}
	}

	emphasis := buildEmphasisMask(len(runes), candidate.FuzzyPositionsRunes(text, queryRunes))

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

func tokenStyle(cat highlighter.TokenCategory, selected bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(appTheme.Text))
	if selected {
		style = style.Background(lipgloss.Color(appTheme.SelectionBG))
	}

	switch cat {
	case highlighter.TokenKeyword:
		return style.Foreground(lipgloss.Color(appTheme.Keyword))
	case highlighter.TokenType:
		return style.Foreground(lipgloss.Color(appTheme.Type))
	case highlighter.TokenFunction:
		return style.Foreground(lipgloss.Color(appTheme.Function))
	case highlighter.TokenString:
		return style.Foreground(lipgloss.Color(appTheme.String))
	case highlighter.TokenNumber:
		return style.Foreground(lipgloss.Color(appTheme.Number))
	case highlighter.TokenComment:
		return style.Foreground(lipgloss.Color(appTheme.Comment))
	case highlighter.TokenOperator:
		return style.Foreground(lipgloss.Color(appTheme.Operator)).Faint(true)
	case highlighter.TokenError:
		return style.Foreground(lipgloss.Color(appTheme.Error)).Bold(true)
	default:
		return style
	}
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
	return idx >= 0 && idx < len(mask) && mask[idx]
}
