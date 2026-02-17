package main

import (
	"container/list"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	bashlang "github.com/smacker/go-tree-sitter/bash"
	clang "github.com/smacker/go-tree-sitter/c"
	cpplang "github.com/smacker/go-tree-sitter/cpp"
	golang "github.com/smacker/go-tree-sitter/golang"
	python "github.com/smacker/go-tree-sitter/python"
	rust "github.com/smacker/go-tree-sitter/rust"
	toml "github.com/smacker/go-tree-sitter/toml"
	tsxlang "github.com/smacker/go-tree-sitter/typescript/tsx"
	tslang "github.com/smacker/go-tree-sitter/typescript/typescript"
	yaml "github.com/smacker/go-tree-sitter/yaml"
	tsjson "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

type LangID string

const (
	LangPlain      LangID = "plain"
	LangGo         LangID = "go"
	LangRust       LangID = "rust"
	LangPython     LangID = "python"
	LangJavaScript LangID = "javascript"
	LangTypeScript LangID = "typescript"
	LangTSX        LangID = "tsx"
	LangYAML       LangID = "yaml"
	LangTOML       LangID = "toml"
	LangJSON       LangID = "json"
	LangBash       LangID = "bash"
	LangC          LangID = "c"
	LangCPP        LangID = "cpp"
)

type HighlightContextMode string

const (
	HighlightContextSynthetic HighlightContextMode = "synthetic"
	HighlightContextFile      HighlightContextMode = "file"
)

func ParseHighlightContextMode(v string) (HighlightContextMode, error) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "", string(HighlightContextSynthetic):
		return HighlightContextSynthetic, nil
	case string(HighlightContextFile):
		return HighlightContextFile, nil
	default:
		return "", fmt.Errorf("invalid highlight context %q (use synthetic or file)", v)
	}
}

var extLangMap = map[string]LangID{
	".go":    LangGo,
	".rs":    LangRust,
	".py":    LangPython,
	".js":    LangJavaScript,
	".jsx":   LangJavaScript,
	".mjs":   LangJavaScript,
	".cjs":   LangJavaScript,
	".ts":    LangTypeScript,
	".tsx":   LangTSX,
	".yaml":  LangYAML,
	".yml":   LangYAML,
	".toml":  LangTOML,
	".json":  LangJSON,
	".jsonc": LangJSON,
	".json5": LangJSON,
	".sh":    LangBash,
	".bash":  LangBash,
	".zsh":   LangBash,
	".c":     LangC,
	".h":     LangC,
	".cpp":   LangCPP,
	".cc":    LangCPP,
	".cxx":   LangCPP,
	".hpp":   LangCPP,
	".hh":    LangCPP,

	".java":  LangPlain,
	".kt":    LangPlain,
	".swift": LangPlain,
	".rb":    LangPlain,
	".php":   LangPlain,
	".lua":   LangPlain,
	".ini":   LangPlain,
	".conf":  LangPlain,
	".md":    LangPlain,
}

var fileLangMap = map[string]LangID{
	"Makefile":          LangPlain,
	"Dockerfile":        LangPlain,
	".bashrc":           LangBash,
	".zshrc":            LangBash,
	".gitignore":        LangPlain,
	".editorconfig":     LangPlain,
	"Cargo.toml":        LangTOML,
	"package-lock.json": LangJSON,
	"go.mod":            LangGo,
	"go.sum":            LangPlain,
}

func DetectLanguage(path string) LangID {
	base := filepath.Base(path)
	if lang, ok := fileLangMap[base]; ok {
		return lang
	}
	ext := strings.ToLower(filepath.Ext(base))
	if lang, ok := extLangMap[ext]; ok {
		return lang
	}
	return LangPlain
}

func DetectLanguageWithShebang(path string, firstLine string) LangID {
	lang := DetectLanguage(path)
	if lang != LangPlain {
		return lang
	}

	if !strings.HasPrefix(firstLine, "#!") {
		return lang
	}
	lower := strings.ToLower(firstLine)
	switch {
	case strings.Contains(lower, "python"):
		return LangPython
	case strings.Contains(lower, "bash") || strings.Contains(lower, "zsh") || strings.Contains(lower, "sh"):
		return LangBash
	case strings.Contains(lower, "node"):
		return LangJavaScript
	default:
		return lang
	}
}

type TokenCategory int

const (
	TokenPlain TokenCategory = iota
	TokenKeyword
	TokenType
	TokenFunction
	TokenString
	TokenNumber
	TokenComment
	TokenOperator
	TokenError
)

type Span struct {
	Start int
	End   int
	Cat   TokenCategory
}

type HighlightRequest struct {
	Lang LangID
	Text string
	File string
	Line int
	Mode HighlightContextMode
}

type cacheKey struct {
	Mode HighlightContextMode
	Lang LangID
	Text string
	File string
	Line int
}

type cacheEntry struct {
	key   cacheKey
	spans []Span
}

type spanLRU struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[cacheKey]*list.Element
}

func newSpanLRU(capacity int) *spanLRU {
	if capacity <= 0 {
		capacity = 1
	}
	return &spanLRU{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[cacheKey]*list.Element, capacity),
	}
}

func (c *spanLRU) Get(key cacheKey) ([]Span, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(elem)
	entry := elem.Value.(cacheEntry)
	return entry.spans, true
}

func (c *spanLRU) Set(key cacheKey, spans []Span) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value = cacheEntry{key: key, spans: spans}
		c.ll.MoveToFront(elem)
		return
	}

	elem := c.ll.PushFront(cacheEntry{key: key, spans: spans})
	c.items[key] = elem

	if c.ll.Len() <= c.capacity {
		return
	}

	back := c.ll.Back()
	if back == nil {
		return
	}
	entry := back.Value.(cacheEntry)
	delete(c.items, entry.key)
	c.ll.Remove(back)
}

type HighlighterConfig struct {
	CacheSize     int
	Workers       int
	Root          string
	DefaultMode   HighlightContextMode
	ContextRadius int
}

type Highlighter struct {
	cache *spanLRU
	tasks chan HighlightRequest

	pendingMu sync.Mutex
	pending   map[cacheKey]struct{}

	langs map[LangID]*sitter.Language

	root          string
	defaultMode   HighlightContextMode
	contextRadius int

	fileMu    sync.RWMutex
	fileLines map[string][]string
}

func NewHighlighter(cfg HighlighterConfig) *Highlighter {
	workers := cfg.Workers
	if workers < 1 {
		workers = 1
	}

	mode := cfg.DefaultMode
	if mode == "" {
		mode = HighlightContextSynthetic
	}

	contextRadius := cfg.ContextRadius
	if contextRadius <= 0 {
		contextRadius = 40
	}

	root := strings.TrimSpace(cfg.Root)
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}

	h := &Highlighter{
		cache:   newSpanLRU(cfg.CacheSize),
		tasks:   make(chan HighlightRequest, workers*256),
		pending: make(map[cacheKey]struct{}),
		langs: map[LangID]*sitter.Language{
			LangGo:         golang.GetLanguage(),
			LangRust:       rust.GetLanguage(),
			LangPython:     python.GetLanguage(),
			LangJavaScript: tslang.GetLanguage(),
			LangTypeScript: tslang.GetLanguage(),
			LangTSX:        tsxlang.GetLanguage(),
			LangYAML:       yaml.GetLanguage(),
			LangTOML:       toml.GetLanguage(),
			LangJSON:       sitter.NewLanguage(tsjson.Language()),
			LangBash:       bashlang.GetLanguage(),
			LangC:          clang.GetLanguage(),
			LangCPP:        cpplang.GetLanguage(),
		},
		root:          root,
		defaultMode:   mode,
		contextRadius: contextRadius,
		fileLines:     make(map[string][]string),
	}

	for i := 0; i < workers; i++ {
		go h.worker()
	}

	return h
}

func (h *Highlighter) Lookup(req HighlightRequest) ([]Span, bool) {
	normalized := h.normalizeRequest(req)
	key := cacheKeyForRequest(normalized)
	return h.cache.Get(key)
}

func (h *Highlighter) Queue(req HighlightRequest) {
	normalized := h.normalizeRequest(req)
	if normalized.Text == "" {
		return
	}

	key := cacheKeyForRequest(normalized)
	if _, ok := h.cache.Get(key); ok {
		return
	}

	h.pendingMu.Lock()
	if _, ok := h.pending[key]; ok {
		h.pendingMu.Unlock()
		return
	}
	h.pending[key] = struct{}{}
	h.pendingMu.Unlock()

	select {
	case h.tasks <- normalized:
	default:
		h.pendingMu.Lock()
		delete(h.pending, key)
		h.pendingMu.Unlock()
	}
}

func (h *Highlighter) worker() {
	parser := sitter.NewParser()

	for req := range h.tasks {
		spans := h.highlightWithParser(parser, req)
		key := cacheKeyForRequest(req)
		h.cache.Set(key, spans)

		h.pendingMu.Lock()
		delete(h.pending, key)
		h.pendingMu.Unlock()
	}
}

func (h *Highlighter) normalizeRequest(req HighlightRequest) HighlightRequest {
	if req.Mode == "" {
		req.Mode = h.defaultMode
	}

	if req.Mode == HighlightContextFile {
		req.File = filepath.Clean(strings.TrimSpace(req.File))
		if req.File != "" && req.Line > 0 {
			if !filepath.IsAbs(req.File) && h.root != "" {
				req.File = filepath.Join(h.root, req.File)
			}
			return req
		}
	}

	req.Mode = HighlightContextSynthetic
	req.File = ""
	req.Line = 0
	return req
}

func cacheKeyForRequest(req HighlightRequest) cacheKey {
	key := cacheKey{
		Mode: req.Mode,
		Lang: req.Lang,
		Text: req.Text,
	}
	if req.Mode == HighlightContextFile {
		key.File = req.File
		key.Line = req.Line
	}
	return key
}

func (h *Highlighter) highlightWithParser(parser *sitter.Parser, req HighlightRequest) []Span {
	if req.Mode == HighlightContextFile {
		if spans, ok := h.highlightFromFileContext(parser, req); ok {
			return spans
		}
	}
	return h.highlightSynthetic(parser, req.Lang, req.Text)
}

func (h *Highlighter) highlightSynthetic(parser *sitter.Parser, lang LangID, text string) []Span {
	runeLen := utf8.RuneCountInString(text)
	if runeLen == 0 {
		return nil
	}

	language, ok := h.langs[lang]
	if !ok || language == nil {
		return plainSpans(text)
	}

	parser.SetLanguage(language)

	source, lineStart, lineEnd := scaffoldLine(lang, text)
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil || tree == nil {
		return plainSpans(text)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return plainSpans(text)
	}

	raw := make([]rawSpan, 0, 32)
	collectLeafSpans(root, lineStart, lineEnd, source, lang, "", "", &raw)
	if len(raw) == 0 {
		return plainSpans(text)
	}

	return buildMergedSpans(text, raw)
}

func (h *Highlighter) highlightFromFileContext(parser *sitter.Parser, req HighlightRequest) ([]Span, bool) {
	language, ok := h.langs[req.Lang]
	if !ok || language == nil {
		return nil, false
	}

	lines, err := h.loadFileLines(req.File)
	if err != nil || len(lines) == 0 {
		return nil, false
	}
	if req.Line < 1 || req.Line > len(lines) {
		return nil, false
	}

	targetLine := lines[req.Line-1]
	display := req.Text
	if display == "" {
		display = targetLine
	}

	startLine := max(1, req.Line-h.contextRadius)
	endLine := min(len(lines), req.Line+h.contextRadius)
	source, targetStart, targetEnd, ok := buildSliceSource(lines[startLine-1:endLine], req.Line-startLine)
	if !ok {
		return nil, false
	}

	parser.SetLanguage(language)
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil || tree == nil {
		return nil, false
	}
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return nil, false
	}

	raw := make([]rawSpan, 0, 32)
	collectLeafSpans(root, targetStart, targetEnd, source, req.Lang, "", "", &raw)
	baseSpans := plainSpans(targetLine)
	if len(raw) > 0 {
		baseSpans = buildMergedSpans(targetLine, raw)
	}

	projected, ok := projectSpansToDisplay(baseSpans, targetLine, display)
	if !ok {
		return nil, false
	}
	return projected, true
}

func (h *Highlighter) loadFileLines(path string) ([]string, error) {
	h.fileMu.RLock()
	if lines, ok := h.fileLines[path]; ok {
		h.fileMu.RUnlock()
		return lines, nil
	}
	h.fileMu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	h.fileMu.Lock()
	h.fileLines[path] = lines
	h.fileMu.Unlock()

	return lines, nil
}

func buildSliceSource(lines []string, targetIndex int) ([]byte, int, int, bool) {
	if targetIndex < 0 || targetIndex >= len(lines) {
		return nil, 0, 0, false
	}

	var sb strings.Builder
	lineStart := 0
	lineEnd := 0

	for i := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if i == targetIndex {
			lineStart = sb.Len()
		}
		sb.WriteString(lines[i])
		if i == targetIndex {
			lineEnd = sb.Len()
		}
	}

	return []byte(sb.String()), lineStart, lineEnd, true
}

func projectSpansToDisplay(baseSpans []Span, sourceLine string, displayLine string) ([]Span, bool) {
	if displayLine == "" {
		return nil, true
	}

	displayRunes := []rune(displayLine)
	if len(displayRunes) == 0 {
		return nil, true
	}

	normalizedSource, normalizedToSource := normalizeLineForDisplayRunes(sourceLine)

	prefixLen := len(displayRunes)
	hasEllipsis := false
	if len(displayRunes) >= 3 && strings.HasSuffix(displayLine, "...") {
		hasEllipsis = true
		prefixLen = len(displayRunes) - 3
	}

	if prefixLen > len(normalizedSource) {
		return nil, false
	}
	if !runesEqual(displayRunes[:prefixLen], normalizedSource[:prefixLen]) {
		return nil, false
	}

	projected := make([]Span, 0, len(baseSpans)+2)
	appendSpan := func(start int, end int, cat TokenCategory) {
		if end <= start {
			return
		}
		if len(projected) > 0 {
			last := &projected[len(projected)-1]
			if last.End == start && last.Cat == cat {
				last.End = end
				return
			}
		}
		projected = append(projected, Span{Start: start, End: end, Cat: cat})
	}

	spanIdx := 0
	for i := 0; i < prefixLen; i++ {
		srcIdx := normalizedToSource[i]
		cat := TokenPlain
		for spanIdx < len(baseSpans) && srcIdx >= baseSpans[spanIdx].End {
			spanIdx++
		}
		if spanIdx < len(baseSpans) {
			span := baseSpans[spanIdx]
			if srcIdx >= span.Start && srcIdx < span.End {
				cat = span.Cat
			}
		}
		appendSpan(i, i+1, cat)
	}

	if hasEllipsis {
		appendSpan(prefixLen, len(displayRunes), TokenPlain)
	}

	return normalizeSpans(projected, len(displayRunes)), true
}

func normalizeLineForDisplayRunes(line string) ([]rune, []int) {
	source := []rune(line)
	out := make([]rune, 0, len(source))
	indexMap := make([]int, 0, len(source))

	for i, r := range source {
		switch r {
		case '\r':
			continue
		case '\n':
			out = append(out, ' ')
			indexMap = append(indexMap, i)
		case '\t':
			for j := 0; j < 4; j++ {
				out = append(out, ' ')
				indexMap = append(indexMap, i)
			}
		default:
			out = append(out, r)
			indexMap = append(indexMap, i)
		}
	}

	return out, indexMap
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

type rawSpan struct {
	Start int
	End   int
	Cat   TokenCategory
}

func collectLeafSpans(node *sitter.Node, lineStart int, lineEnd int, src []byte, lang LangID, parentType string, grandType string, out *[]rawSpan) {
	if node == nil {
		return
	}

	start := int(node.StartByte())
	end := int(node.EndByte())
	if end <= lineStart || start >= lineEnd {
		return
	}

	if node.ChildCount() == 0 {
		clippedStart := max(start, lineStart)
		clippedEnd := min(end, lineEnd)
		if clippedStart >= clippedEnd {
			return
		}

		cat := classifyLeaf(lang, node, parentType, grandType, src[start:end])
		*out = append(*out, rawSpan{
			Start: clippedStart - lineStart,
			End:   clippedEnd - lineStart,
			Cat:   cat,
		})
		return
	}

	nextParent := strings.ToLower(node.Type())
	for i := 0; i < int(node.ChildCount()); i++ {
		collectLeafSpans(node.Child(i), lineStart, lineEnd, src, lang, nextParent, parentType, out)
	}
}

func classifyLeaf(lang LangID, node *sitter.Node, parentType string, grandType string, text []byte) TokenCategory {
	nodeType := strings.ToLower(node.Type())
	parentType = strings.ToLower(parentType)
	grandType = strings.ToLower(grandType)
	lexeme := strings.ToLower(strings.TrimSpace(string(text)))

	if nodeType == "error" || strings.Contains(nodeType, "invalid") {
		return TokenError
	}
	if strings.Contains(nodeType, "comment") {
		return TokenComment
	}
	if strings.Contains(nodeType, "string") || strings.Contains(nodeType, "char") || strings.Contains(nodeType, "heredoc") {
		if lang == LangJSON && (parentType == "pair" || grandType == "pair") {
			return TokenType
		}
		return TokenString
	}
	if strings.Contains(nodeType, "number") || strings.Contains(nodeType, "integer") || strings.Contains(nodeType, "float") || strings.Contains(nodeType, "numeric") {
		return TokenNumber
	}
	if lexeme == "true" || lexeme == "false" || lexeme == "null" || lexeme == "nil" || lexeme == "none" {
		return TokenNumber
	}

	if strings.HasSuffix(nodeType, "keyword") {
		return TokenKeyword
	}

	if strings.Contains(nodeType, "type_identifier") || strings.Contains(nodeType, "primitive_type") || strings.Contains(nodeType, "predefined_type") {
		return TokenType
	}

	if isIdentifierNode(nodeType) {
		if isTypeContext(lang, parentType, grandType) {
			return TokenType
		}
		if isFunctionContext(lang, parentType, grandType) {
			return TokenFunction
		}
		if isLikelyConstant(lexeme) {
			return TokenNumber
		}
	}

	if keywordSet[lexeme] {
		return TokenKeyword
	}
	if operatorSet[lexeme] {
		return TokenOperator
	}

	if !node.IsNamed() {
		if operatorSet[lexeme] || looksLikeOperator(lexeme) {
			return TokenOperator
		}
		if keywordSet[lexeme] || strings.HasSuffix(nodeType, "keyword") {
			return TokenKeyword
		}
	}

	return TokenPlain
}

func isIdentifierNode(nodeType string) bool {
	return nodeType == "identifier" || nodeType == "property_identifier" || strings.HasSuffix(nodeType, "identifier") || strings.HasSuffix(nodeType, "name")
}

func isFunctionContext(lang LangID, parentType string, grandType string) bool {
	if strings.Contains(parentType, "function") || strings.Contains(parentType, "method") || strings.Contains(parentType, "call") || strings.Contains(grandType, "function") || strings.Contains(grandType, "method") || strings.Contains(grandType, "call") {
		return true
	}

	if set, ok := functionContextByLang[lang]; ok && (set[parentType] || set[grandType]) {
		return true
	}
	return false
}

func isTypeContext(lang LangID, parentType string, grandType string) bool {
	if strings.Contains(parentType, "type") || strings.Contains(grandType, "type") || strings.Contains(parentType, "class") || strings.Contains(parentType, "struct") || strings.Contains(parentType, "interface") || strings.Contains(parentType, "trait") || strings.Contains(grandType, "class") || strings.Contains(grandType, "struct") || strings.Contains(grandType, "interface") || strings.Contains(grandType, "trait") {
		return true
	}

	if set, ok := typeContextByLang[lang]; ok && (set[parentType] || set[grandType]) {
		return true
	}
	return false
}

func isLikelyConstant(s string) bool {
	if len(s) < 2 {
		return false
	}
	hasLetter := false
	for _, r := range s {
		switch {
		case r == '_':
			continue
		case unicode.IsDigit(r):
			continue
		case unicode.IsLetter(r):
			hasLetter = true
			if unicode.IsLower(r) {
				return false
			}
		default:
			return false
		}
	}
	return hasLetter
}

func looksLikeOperator(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch r {
		case '+', '-', '*', '/', '%', '=', '!', '<', '>', '&', '|', '^', '~', ':', ';', ',', '.', '?', '(', ')', '[', ']', '{', '}':
		default:
			return false
		}
	}
	return true
}

func scaffoldLine(lang LangID, line string) ([]byte, int, int) {
	prefix := ""
	suffix := "\n"

	switch lang {
	case LangGo:
		prefix = "package p\nfunc _snav_() {\n"
		suffix = "\n}\n"
	case LangRust:
		prefix = "fn _snav_() {\n"
		suffix = "\n}\n"
	case LangJavaScript, LangTypeScript, LangTSX:
		prefix = "function _snav_() {\n"
		suffix = "\n}\n"
	case LangC, LangCPP:
		prefix = "void _snav_() {\n"
		suffix = "\n}\n"
	case LangJSON:
		prefix = "{\n"
		suffix = "\n}\n"
	}

	source := []byte(prefix + line + suffix)
	start := len(prefix)
	end := start + len(line)
	return source, start, end
}

func plainSpans(text string) []Span {
	runeLen := utf8.RuneCountInString(text)
	if runeLen == 0 {
		return nil
	}
	return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
}

func buildMergedSpans(text string, raw []rawSpan) []Span {
	runeLen := utf8.RuneCountInString(text)
	if runeLen == 0 {
		return nil
	}
	if len(raw) == 0 {
		return plainSpans(text)
	}

	sort.Slice(raw, func(i, j int) bool {
		if raw[i].Start == raw[j].Start {
			return raw[i].End < raw[j].End
		}
		return raw[i].Start < raw[j].Start
	})

	spans := make([]Span, 0, len(raw)+2)
	for _, rs := range raw {
		startRune := byteToRuneIndex(text, rs.Start)
		endRune := byteToRuneIndex(text, rs.End)
		if endRune <= startRune {
			continue
		}
		spans = append(spans, Span{Start: startRune, End: endRune, Cat: rs.Cat})
	}

	return normalizeSpans(spans, runeLen)
}

func normalizeSpans(spans []Span, runeLen int) []Span {
	if runeLen <= 0 {
		return nil
	}
	if len(spans) == 0 {
		return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
	}

	clean := make([]Span, 0, len(spans))
	for _, span := range spans {
		start := span.Start
		end := span.End
		if start < 0 {
			start = 0
		}
		if end > runeLen {
			end = runeLen
		}
		if end <= start {
			continue
		}
		clean = append(clean, Span{Start: start, End: end, Cat: span.Cat})
	}

	if len(clean) == 0 {
		return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
	}

	sort.Slice(clean, func(i, j int) bool {
		if clean[i].Start == clean[j].Start {
			return clean[i].End < clean[j].End
		}
		return clean[i].Start < clean[j].Start
	})

	out := make([]Span, 0, len(clean)+2)
	cursor := 0
	for _, span := range clean {
		start := span.Start
		end := span.End

		if start < cursor {
			start = cursor
		}
		if end <= start {
			continue
		}

		if start > cursor {
			out = append(out, Span{Start: cursor, End: start, Cat: TokenPlain})
		}

		if len(out) > 0 {
			last := &out[len(out)-1]
			if last.End == start && last.Cat == span.Cat {
				last.End = end
			} else {
				out = append(out, Span{Start: start, End: end, Cat: span.Cat})
			}
		} else {
			out = append(out, Span{Start: start, End: end, Cat: span.Cat})
		}

		cursor = end
	}

	if cursor < runeLen {
		out = append(out, Span{Start: cursor, End: runeLen, Cat: TokenPlain})
	}

	if len(out) == 0 {
		return []Span{{Start: 0, End: runeLen, Cat: TokenPlain}}
	}

	return out
}

func byteToRuneIndex(s string, b int) int {
	if b <= 0 {
		return 0
	}
	if b >= len(s) {
		return utf8.RuneCountInString(s)
	}
	return utf8.RuneCountInString(s[:b])
}

var functionContextByLang = map[LangID]map[string]bool{
	LangGo: {
		"function_declaration": true,
		"method_declaration":   true,
		"call_expression":      true,
		"selector_expression":  true,
	},
	LangRust: {
		"function_item":    true,
		"call_expression":  true,
		"field_expression": true,
	},
	LangJavaScript: {
		"function_declaration": true,
		"method_definition":    true,
		"call_expression":      true,
		"member_expression":    true,
	},
	LangTypeScript: {
		"function_declaration": true,
		"method_definition":    true,
		"call_expression":      true,
		"member_expression":    true,
	},
	LangTSX: {
		"function_declaration": true,
		"method_definition":    true,
		"call_expression":      true,
		"member_expression":    true,
	},
	LangPython: {
		"function_definition": true,
		"call":                true,
	},
	LangC: {
		"function_definition": true,
		"call_expression":     true,
	},
	LangCPP: {
		"function_definition": true,
		"call_expression":     true,
	},
}

var typeContextByLang = map[LangID]map[string]bool{
	LangGo: {
		"type_spec":             true,
		"type_declaration":      true,
		"parameter_declaration": true,
		"var_declaration":       true,
	},
	LangRust: {
		"struct_item": true,
		"enum_item":   true,
		"trait_item":  true,
		"type_item":   true,
	},
	LangJavaScript: {
		"class_declaration": true,
		"type_annotation":   true,
	},
	LangTypeScript: {
		"interface_declaration":  true,
		"type_alias_declaration": true,
		"type_annotation":        true,
		"class_declaration":      true,
	},
	LangTSX: {
		"interface_declaration":  true,
		"type_alias_declaration": true,
		"type_annotation":        true,
		"class_declaration":      true,
	},
	LangPython: {
		"class_definition": true,
	},
}

var keywordSet = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "case": true,
	"catch": true, "class": true, "const": true, "continue": true, "def": true,
	"default": true, "defer": true, "do": true, "else": true, "enum": true,
	"export": true, "extends": true, "fallthrough": true, "finally": true,
	"fn": true, "for": true, "from": true, "func": true, "function": true,
	"if": true, "impl": true, "import": true, "in": true, "include": true,
	"interface": true, "let": true, "loop": true, "match": true, "mod": true,
	"module": true, "mut": true, "namespace": true, "new": true, "package": true,
	"pub": true, "raise": true, "return": true, "struct": true, "switch": true,
	"trait": true, "try": true, "type": true, "use": true, "var": true,
	"while": true, "with": true, "yield": true,
}

var operatorSet = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true,
	"=": true, "==": true, "!=": true, "<": true, "<=": true,
	">": true, ">=": true, "&&": true, "||": true, "!": true,
	"&": true, "|": true, "^": true, "~": true, "->": true,
	"=>": true, "::": true, ":": true, ";": true, ",": true,
	".": true, "?": true, "(": true, ")": true, "[": true,
	"]": true, "{": true, "}": true,
}
