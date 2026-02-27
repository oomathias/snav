package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	chroma "github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

type ThemePalette struct {
	Name        string
	Text        string
	InputBG     string
	SelectionBG string
	Muted       string
	Dim         string
	PathDir     string
	PathFile    string
	PathMeta    string
	Header      string
	Accent      string
	Keyword     string
	Type        string
	Function    string
	String      string
	Number      string
	Comment     string
	Operator    string
	Error       string
}

var appTheme = mustDefaultTheme()

func SetTheme(name string) error {
	palette, err := LoadThemePalette(name)
	if err != nil {
		return err
	}
	appTheme = palette
	return nil
}

func LoadThemePalette(name string) (ThemePalette, error) {
	requested := strings.TrimSpace(name)
	if requested == "" {
		requested = "nord"
	}

	lookup := normalizeThemeName(requested)
	names := styles.Names()
	available := make(map[string]struct{}, len(names))
	for _, n := range names {
		available[n] = struct{}{}
	}
	unknownThemeErr := func() error {
		sort.Strings(names)
		return fmt.Errorf("unknown theme %q. try one of: %s", requested, strings.Join(topThemeHints(names), ", "))
	}
	if _, ok := available[lookup]; !ok {
		return ThemePalette{}, unknownThemeErr()
	}

	style := styles.Get(lookup)
	if style == nil {
		return ThemePalette{}, unknownThemeErr()
	}

	baseBG := pickBackground(style, "#2E3440", chroma.Background, chroma.LineHighlight)
	baseFG := pickForeground(style, "#D8DEE9", chroma.Text, chroma.Background)
	comment := pickForeground(style, adjustTone(baseFG, -60), chroma.Comment)

	selectionBG := pickBackground(style, autoSelection(baseBG), chroma.LineHighlight)
	inputBG := adjustTone(baseBG, autoDelta(baseBG, 12, -12))

	palette := ThemePalette{
		Name:        lookup,
		Text:        baseFG,
		InputBG:     inputBG,
		SelectionBG: selectionBG,
		Muted:       pickForeground(style, adjustTone(baseFG, -48), chroma.LineNumbers, chroma.Comment),
		Dim:         pickForeground(style, adjustTone(comment, -10), chroma.Comment),
		PathDir:     pickForeground(style, adjustTone(comment, 0), chroma.Comment),
		PathFile:    pickForeground(style, adjustTone(baseFG, -30), chroma.Name, chroma.NameNamespace),
		PathMeta:    pickForeground(style, adjustTone(baseFG, -40), chroma.Comment, chroma.Text),
		Header:      pickForeground(style, adjustTone(baseFG, -20), chroma.NameClass, chroma.Keyword),
		Accent:      pickForeground(style, baseFG, chroma.NameFunction, chroma.Keyword),
		Keyword:     pickForeground(style, baseFG, chroma.Keyword),
		Type:        pickForeground(style, baseFG, chroma.KeywordType, chroma.NameClass),
		Function:    pickForeground(style, baseFG, chroma.NameFunction, chroma.Name),
		String:      pickForeground(style, baseFG, chroma.LiteralString),
		Number:      pickForeground(style, baseFG, chroma.LiteralNumber),
		Comment:     comment,
		Operator:    pickForeground(style, baseFG, chroma.Operator),
		Error:       pickForeground(style, "#BF616A", chroma.Error),
	}

	return palette, nil
}

func normalizeThemeName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "solarized":
		return "solarized-dark"
	case "one-dark":
		return "onedark"
	default:
		return n
	}
}

func pickForeground(style *chroma.Style, fallback string, types ...chroma.TokenType) string {
	for _, tt := range types {
		entry := style.Get(tt)
		if entry.Colour.IsSet() {
			return entry.Colour.String()
		}
	}
	return fallback
}

func pickBackground(style *chroma.Style, fallback string, types ...chroma.TokenType) string {
	for _, tt := range types {
		entry := style.Get(tt)
		if entry.Background.IsSet() {
			return entry.Background.String()
		}
	}
	return fallback
}

func topThemeHints(all []string) []string {
	wanted := []string{"nord", "dracula", "monokai", "github", "github-dark", "solarized-dark", "solarized-light", "gruvbox", "onedark"}
	set := map[string]bool{}
	for _, n := range all {
		set[n] = true
	}
	out := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if set[name] {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		limit := min(8, len(all))
		return all[:limit]
	}
	return out
}

func autoSelection(bg string) string {
	return adjustTone(bg, autoDelta(bg, 18, -18))
}

func autoDelta(bg string, darkDelta int, lightDelta int) int {
	r, g, b, ok := parseHexRGB(bg)
	if !ok {
		return darkDelta
	}
	l := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
	if l < 128 {
		return darkDelta
	}
	return lightDelta
}

func adjustTone(hex string, delta int) string {
	r, g, b, ok := parseHexRGB(hex)
	if !ok {
		return hex
	}
	r = clamp8(r + delta)
	g = clamp8(g + delta)
	b = clamp8(b + delta)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func parseHexRGB(hex string) (int, int, int, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	r := int((v >> 16) & 0xFF)
	g := int((v >> 8) & 0xFF)
	b := int(v & 0xFF)
	return r, g, b, true
}

func clamp8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func mustDefaultTheme() ThemePalette {
	p, err := LoadThemePalette("nord")
	if err == nil {
		return p
	}
	return ThemePalette{
		Name:        "fallback",
		Text:        "#D8DEE9",
		InputBG:     "#3B4252",
		SelectionBG: "#434C5E",
		Muted:       "#4C566A",
		Dim:         "#4C566A",
		PathDir:     "#4C566A",
		PathFile:    "#7B8598",
		PathMeta:    "#6B7280",
		Header:      "#8FBCBB",
		Accent:      "#88C0D0",
		Keyword:     "#81A1C1",
		Type:        "#8FBCBB",
		Function:    "#88C0D0",
		String:      "#A3BE8C",
		Number:      "#B48EAD",
		Comment:     "#4C566A",
		Operator:    "#D8DEE9",
		Error:       "#BF616A",
	}
}
