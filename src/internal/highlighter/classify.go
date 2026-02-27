package highlighter

import (
	"strings"
	"unicode"

	sitter "github.com/smacker/go-tree-sitter"
)

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
	if containsAnySubstring(nodeType, "string", "char", "heredoc") {
		if lang == LangJSON && (parentType == "pair" || grandType == "pair") {
			return TokenType
		}
		return TokenString
	}
	if containsAnySubstring(nodeType, "number", "integer", "float", "numeric") {
		return TokenNumber
	}
	if lexeme == "true" || lexeme == "false" || lexeme == "null" || lexeme == "nil" || lexeme == "none" {
		return TokenNumber
	}

	if strings.HasSuffix(nodeType, "keyword") {
		return TokenKeyword
	}

	if containsAnySubstring(nodeType, "type_identifier", "primitive_type", "predefined_type") {
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

	if !node.IsNamed() && looksLikeOperator(lexeme) {
		return TokenOperator
	}

	return TokenPlain
}

func isIdentifierNode(nodeType string) bool {
	return nodeType == "identifier" || nodeType == "property_identifier" || strings.HasSuffix(nodeType, "identifier") || strings.HasSuffix(nodeType, "name")
}

func isFunctionContext(lang LangID, parentType string, grandType string) bool {
	return isContext(lang, parentType, grandType, functionContextByLang, "function", "method", "call")
}

func isTypeContext(lang LangID, parentType string, grandType string) bool {
	return isContext(lang, parentType, grandType, typeContextByLang, "type", "class", "struct", "interface", "trait")
}

func isContext(lang LangID, parentType string, grandType string, byLang map[LangID]map[string]bool, hints ...string) bool {
	if containsAnySubstring(parentType, hints...) || containsAnySubstring(grandType, hints...) {
		return true
	}

	set := byLang[lang]
	return set[parentType] || set[grandType]
}

func containsAnySubstring(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
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
	LangJavaScript: jsLikeFunctionContexts,
	LangTypeScript: jsLikeFunctionContexts,
	LangTSX:        jsLikeFunctionContexts,
	LangPython: {
		"function_definition": true,
		"call":                true,
	},
	LangC:   cFamilyFunctionContexts,
	LangCPP: cFamilyFunctionContexts,
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
	LangTypeScript: tsLikeTypeContexts,
	LangTSX:        tsLikeTypeContexts,
	LangPython: {
		"class_definition": true,
	},
}

var jsLikeFunctionContexts = map[string]bool{
	"function_declaration": true,
	"method_definition":    true,
	"call_expression":      true,
	"member_expression":    true,
}

var cFamilyFunctionContexts = map[string]bool{
	"function_definition": true,
	"call_expression":     true,
}

var tsLikeTypeContexts = map[string]bool{
	"interface_declaration":  true,
	"type_alias_declaration": true,
	"type_annotation":        true,
	"class_declaration":      true,
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
