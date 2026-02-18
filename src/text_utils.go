package main

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

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
	return utf8.RuneCountInString(s)
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

func shouldUseIncrementalFilter(current []rune, previous []rune, candidateN int, previousCandidateN int) bool {
	if len(current) == 0 || len(previous) == 0 {
		return false
	}
	if len(current) <= len(previous) {
		return false
	}
	if candidateN != previousCandidateN {
		return false
	}
	if len(previous) > len(current) {
		return false
	}
	for i := range previous {
		if current[i] != previous[i] {
			return false
		}
	}
	return true
}

func copyRunesReuse(dst []rune, src []rune) []rune {
	if len(src) == 0 {
		return nil
	}
	if cap(dst) < len(src) {
		dst = make([]rune, len(src))
	} else {
		dst = dst[:len(src)]
	}
	copy(dst, src)
	return dst
}

func runesEqual(a []rune, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
