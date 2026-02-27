package candidate

import "strings"

const (
	semanticTypeDeclScore    int16 = 460
	semanticConstructorScore int16 = 410
	semanticFunctionScore    int16 = 340
	semanticMethodScore      int16 = 300
	semanticConstScore       int16 = 260
	semanticModuleScore      int16 = 220
	semanticFieldScore       int16 = 170
	semanticLocalScore       int16 = 110
	semanticParamScore       int16 = 80

	semanticVisibilityPublic   int16 = 35
	semanticVisibilityInternal int16 = 20
	semanticVisibilityPrivate  int16 = -15
)

func candidateSemanticScore(cand *Candidate) int16 {
	if cand == nil {
		return 0
	}
	if cand.SemanticScore != 0 {
		return cand.SemanticScore
	}
	return computeSemanticScore(cand.Text)
}

func computeSemanticScore(text string) int16 {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return 0
	}

	keyword, rest, visibility := classifyDeclaration(lower)
	if keyword == "" {
		return visibility
	}

	base := semanticScoreForDeclaration(keyword, rest)
	if base == 0 {
		return visibility
	}
	return base + visibility
}

func classifyDeclaration(lower string) (string, string, int16) {
	remaining := strings.TrimLeft(lower, " \t")
	visibility := int16(0)

	for remaining != "" {
		token, tail := leadingToken(remaining)
		if token == "" {
			return "", "", visibility
		}

		switch token {
		case "export", "public", "pub":
			if visibility < semanticVisibilityPublic {
				visibility = semanticVisibilityPublic
			}
			remaining = trimModifierTail(token, tail)
		case "protected", "internal":
			if visibility < semanticVisibilityInternal {
				visibility = semanticVisibilityInternal
			}
			remaining = trimModifierTail(token, tail)
		case "private":
			if visibility == 0 {
				visibility = semanticVisibilityPrivate
			}
			remaining = trimModifierTail(token, tail)
		case "default", "async", "abstract", "final", "sealed", "partial", "static", "inline", "open", "virtual", "override", "readonly", "extern", "unsafe":
			remaining = trimModifierTail(token, tail)
		default:
			return token, strings.TrimLeft(tail, " \t"), visibility
		}
	}

	return "", "", visibility
}

func trimModifierTail(token string, tail string) string {
	rest := strings.TrimLeft(tail, " \t")
	if token == "pub" && strings.HasPrefix(rest, "(") {
		if end := strings.IndexByte(rest, ')'); end >= 0 {
			rest = rest[end+1:]
		}
	}
	return strings.TrimLeft(rest, " \t")
}

func semanticScoreForDeclaration(keyword string, rest string) int16 {
	switch keyword {
	case "class", "struct", "interface", "enum", "trait", "protocol", "record", "type":
		return semanticTypeDeclScore
	case "constructor":
		return semanticConstructorScore
	case "func", "function", "def", "fn", "fun":
		return semanticFunctionLikeScore(keyword, rest)
	case "const", "static":
		return semanticConstScore
	case "namespace", "module", "mod", "package", "impl", "extension":
		return semanticModuleScore
	case "field", "property":
		return semanticFieldScore
	case "let", "var", "val":
		return semanticLocalScore
	case "param", "parameter":
		return semanticParamScore
	default:
		return 0
	}
}

func semanticFunctionLikeScore(keyword string, rest string) int16 {
	name, isMethod := functionNameAndMethod(keyword, rest)
	if isConstructorName(name) {
		return semanticConstructorScore
	}
	if isMethod {
		return semanticMethodScore
	}
	return semanticFunctionScore
}

func functionNameAndMethod(keyword string, rest string) (string, bool) {
	body := strings.TrimLeft(rest, " \t")
	isMethod := false

	if keyword == "func" && strings.HasPrefix(body, "(") {
		isMethod = true
		if close := strings.IndexByte(body, ')'); close >= 0 {
			body = strings.TrimLeft(body[close+1:], " \t")
		}
	}

	name, after := leadingToken(body)
	after = strings.TrimLeft(after, " \t")

	if keyword == "def" {
		if strings.Contains(after, "(self") || strings.Contains(after, "(cls") {
			isMethod = true
		}
	}
	if keyword == "fn" {
		if strings.Contains(after, "&self") || strings.Contains(after, " self") {
			isMethod = true
		}
	}

	return name, isMethod
}

func isConstructorName(name string) bool {
	if name == "" {
		return false
	}
	if name == "constructor" || name == "__init__" || name == "new" {
		return true
	}
	return hasSemanticPrefix(name, "new") ||
		hasSemanticPrefix(name, "create") ||
		hasSemanticPrefix(name, "make") ||
		hasSemanticPrefix(name, "build") ||
		hasSemanticPrefix(name, "init")
}

func hasSemanticPrefix(name string, prefix string) bool {
	if len(name) <= len(prefix) {
		return false
	}
	return strings.HasPrefix(name, prefix)
}

func leadingToken(s string) (string, string) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return "", ""
	}

	i := 0
	for i < len(s) {
		b := s[i]
		if (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return "", s
	}
	return s[:i], s[i:]
}
