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
