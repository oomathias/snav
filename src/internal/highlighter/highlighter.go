package highlighter

import (
	"container/list"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"snav/internal/lang"
	"snav/internal/readfile"

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

type LangID = lang.ID

const (
	LangPlain      LangID = lang.Plain
	LangGo         LangID = lang.Go
	LangRust       LangID = lang.Rust
	LangPython     LangID = lang.Python
	LangJavaScript LangID = lang.JavaScript
	LangTypeScript LangID = lang.TypeScript
	LangTSX        LangID = lang.TSX
	LangYAML       LangID = lang.YAML
	LangTOML       LangID = lang.TOML
	LangJSON       LangID = lang.JSON
	LangBash       LangID = lang.Bash
	LangC          LangID = lang.C
	LangCPP        LangID = lang.CPP
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
		spans := h.HighlightWithParser(parser, req)
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

func (h *Highlighter) HighlightWithParser(parser *sitter.Parser, req HighlightRequest) []Span {
	if req.Mode == HighlightContextFile {
		if spans, ok := h.highlightFromFileContext(parser, req); ok {
			return spans
		}
	}
	return h.highlightSynthetic(parser, req.Lang, req.Text)
}

func (h *Highlighter) highlightSynthetic(parser *sitter.Parser, lang LangID, text string) []Span {
	if text == "" {
		return nil
	}

	language, ok := h.langs[lang]
	if !ok || language == nil {
		return plainSpans(text)
	}

	source, lineStart, lineEnd := scaffoldLine(lang, text)
	raw, ok := collectRawSpans(parser, language, source, lineStart, lineEnd, lang)
	if !ok {
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

	raw, ok := collectRawSpans(parser, language, source, targetStart, targetEnd, req.Lang)
	if !ok {
		return nil, false
	}
	baseSpans := buildMergedSpans(targetLine, raw)

	projected, ok := projectSpansToDisplay(baseSpans, targetLine, display)
	if !ok {
		return nil, false
	}
	return projected, true
}

func collectRawSpans(parser *sitter.Parser, language *sitter.Language, source []byte, lineStart int, lineEnd int, lang LangID) ([]rawSpan, bool) {
	parser.SetLanguage(language)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil || tree == nil {
		return nil, false
	}

	root := tree.RootNode()
	if root == nil {
		tree.Close()
		return nil, false
	}
	defer tree.Close()

	raw := make([]rawSpan, 0, 32)
	collectLeafSpans(root, lineStart, lineEnd, source, lang, "", "", &raw)
	return raw, true
}

func (h *Highlighter) loadFileLines(path string) ([]string, error) {
	h.fileMu.RLock()
	if lines, ok := h.fileLines[path]; ok {
		h.fileMu.RUnlock()
		return lines, nil
	}
	h.fileMu.RUnlock()

	lines, err := readfile.ReadLinesNormalized(path)
	if err != nil {
		return nil, err
	}

	h.fileMu.Lock()
	h.fileLines[path] = lines
	h.fileMu.Unlock()

	return lines, nil
}
