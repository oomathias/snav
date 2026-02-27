package candidate

import (
	"path/filepath"
	"regexp"
	"strings"
)

var keyRegexes = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:export\s+)?(?:inline\s+)?namespace\s+([A-Za-z_][A-Za-z0-9_]*(?:(?:::|\.)[A-Za-z_][A-Za-z0-9_]*)*)\b`),
	regexp.MustCompile(`^\s*(?:(?:export|default|async|public|private|protected|internal|abstract|final|sealed|partial|static)\s+)*(?:function|class|interface|type|enum|record)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`),
	regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)`),
	regexp.MustCompile(`^\s*func\s*(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`),
	regexp.MustCompile(`^\s*(?:type|var|const)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:fn|struct|enum|trait|mod|type|const|static)\s+([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`^\s*(?:async\s+def|def|class)\s+([A-Za-z_][A-Za-z0-9_]*)`),
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
	for _, re := range keyRegexes {
		m := re.FindStringSubmatch(text)
		if len(m) > 1 {
			for _, group := range m[1:] {
				if group != "" {
					return group
				}
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

func fileBaseWithoutExt(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

var matcherStopWords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "return": true,
	"case": true, "break": true, "continue": true, "default": true,
	"func": true, "type": true, "const": true, "var": true,
	"class": true, "interface": true, "enum": true,
	"namespace": true,
	"export":    true,
	"public":    true, "private": true, "protected": true, "internal": true,
	"abstract": true, "final": true, "sealed": true, "partial": true,
	"static": true, "inline": true,
	"def": true, "fn": true,
}
