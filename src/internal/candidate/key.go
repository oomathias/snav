package candidate

import (
	"path/filepath"
	"regexp"
	"strings"
)

var keyRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:export\s+)?(?:inline\s+)?namespace\s+([A-Za-z_][A-Za-z0-9_]*(?:(?:::|\.)[A-Za-z_][A-Za-z0-9_]*)*)\b`),
	regexp.MustCompile(`^\s*(?:module|package)\s+([A-Za-z_][A-Za-z0-9_]*(?:(?:::|\.)[A-Za-z_][A-Za-z0-9_]*)*)\b`),
	regexp.MustCompile(`^\s*(?:(?:export|default|async|public|private|protected|internal|abstract|final|sealed|partial|static)\s+)*(?:function|class|interface|type|enum|record)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
	regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`^\s*func\s*(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
	regexp.MustCompile(`^\s*(?:type|var|const)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:fn|struct|enum|trait|mod|type|const|static)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*test\s+"((?:\\.|[^"\\])+)"`),
	regexp.MustCompile(`^\s*(?:async\s+def|def|class)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|static|final|abstract|virtual|override|async|extern|unsafe|sealed|partial|readonly|synchronized|native|strictfp)\s+)+(?:[A-Za-z_][A-Za-z0-9_<>,.?\[\]]*\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
	regexp.MustCompile(`^\s*(?:(?:public|private|protected|internal|abstract|final|sealed|partial|static|open|override|data|inline|suspend|tailrec|operator|infix|external|lateinit|const|inner|annotation|enum|value)\s+)*(?:fun|object|class|interface|typealias)\s+([A-Za-z_][A-Za-z0-9_]*)\b`),
	regexp.MustCompile(`^\s*(?:interface|class|enum|record)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:fun|val|var|object|class|interface)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*\[\[([A-Za-z0-9_.:-]+)\]\]\s*$`),
	regexp.MustCompile(`^\s*\[([A-Za-z0-9_.:-]+)\]\s*$`),
	regexp.MustCompile(`^\s*"((?:\\.|[^"\\])+)"\s*:`),
	regexp.MustCompile(`^\s*'([^']+)'\s*:`),
	regexp.MustCompile(`^\s*-\s*"((?:\\.|[^"\\])+)"\s*:`),
	regexp.MustCompile(`^\s*-\s*'([^']+)'\s*:`),
	regexp.MustCompile(`^\s*-\s*([A-Za-z0-9_.-]+)\s*:`),
	regexp.MustCompile(`^\s*export\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*=`),
	regexp.MustCompile(`^\s*[A-Za-z0-9_.-]+\s+"(?:\\.|[^"\\])+"\s+"((?:\\.|[^"\\])+)"\s*\{`),
	regexp.MustCompile(`^\s*[A-Za-z0-9_.-]+\s+"((?:\\.|[^"\\])+)"\s*\{`),
	regexp.MustCompile(`^\s*<[^>]*\b(?:[Kk][Ee][Yy]|[Nn][Aa][Mm][Ee]|[Ii][Dd])\s*=\s*"((?:\\.|[^"\\])+)"`),
	regexp.MustCompile(`^\s*<[^>]*\b(?:[Kk][Ee][Yy]|[Nn][Aa][Mm][Ee]|[Ii][Dd])\s*=\s*'([^']+)'`),
	regexp.MustCompile(`^\s*<\s*([A-Za-z_][A-Za-z0-9_.:-]*)\b`),
	regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*\{`),
	regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*(?::|=)`),
	regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:=`),
	regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:`),
}

var firstIdentifier = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func ExtractKey(text string, file string) string {
	return extractKeyWithConfigHint(text, file, looksLikeConfigFile(file))
}

func extractKeyWithConfigHint(text string, file string, isConfig bool) string {
	if isConfig {
		if key, ok := extractConfigKeyFast(text); ok {
			return key
		}
	}
	if key, ok := extractFunctionKeyFast(text); ok {
		return key
	}

	for _, re := range keyRegexes {
		idx := re.FindStringSubmatchIndex(text)
		for i := 2; i+1 < len(idx); i += 2 {
			if idx[i] >= 0 {
				return text[idx[i]:idx[i+1]]
			}
		}
	}

	if ident := firstIdentifier.FindString(text); ident != "" && !matcherStopWords[strings.ToLower(ident)] {
		return ident
	}

	base := fileBaseWithoutExt(file)
	if base == "" {
		return file
	}
	return base
}

func extractFunctionKeyFast(text string) (string, bool) {
	line := strings.TrimLeft(text, " \t")
	open := strings.IndexByte(line, '(')
	if open <= 0 {
		return "", false
	}

	head := strings.TrimRight(line[:open], " \t")
	if head == "" {
		return "", false
	}

	start, end := lastIdentifierSpan(head)
	if start < 0 || end <= start {
		return "", false
	}

	name := head[start:end]
	if matcherStopWords[strings.ToLower(name)] {
		return "", false
	}
	return name, true
}

func lastIdentifierSpan(s string) (int, int) {
	end := len(s)
	for end > 0 {
		b := s[end-1]
		if b == ' ' || b == '\t' || b == '&' || b == '*' || b == ':' || b == '.' {
			end--
			continue
		}
		break
	}
	if end == 0 {
		return -1, -1
	}

	start := end
	for start > 0 {
		b := s[start-1]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '$' || b == '~' {
			start--
			continue
		}
		break
	}
	if start == end {
		return -1, -1
	}

	return start, end
}

func looksLikeConfigFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)

	switch ext {
	case ".json", ".jsonc", ".json5", ".yaml", ".yml", ".toml", ".ini", ".properties", ".conf", ".cfg", ".cnf", ".tf", ".hcl", ".tfvars", ".xml", ".plist", ".csproj", ".props", ".targets", ".config":
		return true
	}

	if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".envrc" {
		return true
	}

	return false
}

func extractConfigKeyFast(text string) (string, bool) {
	line := strings.TrimLeft(text, " \t")
	if line == "" {
		return "", false
	}

	switch line[0] {
	case '[':
		if strings.HasPrefix(line, "[[") {
			if end := strings.Index(line[2:], "]]"); end >= 0 {
				return line[2 : 2+end], true
			}
			return "", false
		}
		if end := strings.IndexByte(line[1:], ']'); end >= 0 {
			return line[1 : 1+end], true
		}
		return "", false
	case '"', '\'':
		if key, ok := extractQuotedConfigKey(line); ok {
			return key, true
		}
	case '-':
		return extractDashConfigKey(line[1:])
	case '<':
		if key, ok := extractXMLConfigKey(line); ok {
			return key, true
		}
	}

	if strings.HasPrefix(line, "export ") {
		if key, ok := extractSimpleConfigKey(strings.TrimLeft(line[len("export "):], " \t")); ok {
			return key, true
		}
	}

	return extractSimpleConfigKey(line)
}

func extractQuotedConfigKey(line string) (string, bool) {
	quote := line[0]
	end := 1
	for end < len(line) {
		if line[end] == quote && line[end-1] != '\\' {
			break
		}
		end++
	}
	if end >= len(line) {
		return "", false
	}

	rest := strings.TrimLeft(line[end+1:], " \t")
	if rest == "" || rest[0] != ':' {
		return "", false
	}
	return line[1:end], true
}

func extractDashConfigKey(line string) (string, bool) {
	line = strings.TrimLeft(line, " \t")
	if line == "" {
		return "", false
	}
	if line[0] == '"' || line[0] == '\'' {
		return extractQuotedConfigKey(line)
	}
	return extractSimpleConfigKey(line)
}

func extractXMLConfigKey(line string) (string, bool) {
	for _, attr := range []string{"key=\"", "name=\"", "id=\""} {
		if idx := strings.Index(strings.ToLower(line), attr); idx >= 0 {
			start := idx + len(attr)
			if end := strings.IndexByte(line[start:], '"'); end >= 0 {
				return line[start : start+end], true
			}
		}
	}
	for _, attr := range []string{"key='", "name='", "id='"} {
		if idx := strings.Index(strings.ToLower(line), attr); idx >= 0 {
			start := idx + len(attr)
			if end := strings.IndexByte(line[start:], '\''); end >= 0 {
				return line[start : start+end], true
			}
		}
	}

	start := 1
	for start < len(line) && (line[start] == ' ' || line[start] == '\t' || line[start] == '/') {
		start++
	}
	end := start
	for end < len(line) {
		b := line[end]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '.' || b == ':' || b == '-' {
			end++
			continue
		}
		break
	}
	if end == start {
		return "", false
	}
	return line[start:end], true
}

func extractSimpleConfigKey(line string) (string, bool) {
	end := 0
	for end < len(line) {
		b := line[end]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '.' || b == '-' {
			end++
			continue
		}
		break
	}
	if end == 0 {
		return "", false
	}

	rest := strings.TrimLeft(line[end:], " \t")
	if rest == "" {
		return "", false
	}
	if rest[0] != ':' && rest[0] != '=' && rest[0] != '{' {
		return "", false
	}

	return line[:end], true
}

func fileBaseWithoutExt(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

var matcherStopWords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"case": true, "break": true, "continue": true, "default": true,
	"func": true, "type": true, "const": true, "var": true,
	"class": true, "interface": true, "enum": true,
	"namespace": true, "module": true, "package": true,
	"export": true,
	"public": true, "private": true, "protected": true, "internal": true,
	"abstract": true, "final": true, "sealed": true, "partial": true,
	"static": true, "inline": true,
	"pub": true,
	"def": true, "fn": true, "fun": true, "object": true, "typealias": true, "test": true,
}
