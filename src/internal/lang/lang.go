package lang

import (
	"path/filepath"
	"strings"
)

type ID string

const (
	Plain      ID = "plain"
	Go         ID = "go"
	Rust       ID = "rust"
	Python     ID = "python"
	JavaScript ID = "javascript"
	TypeScript ID = "typescript"
	TSX        ID = "tsx"
	YAML       ID = "yaml"
	TOML       ID = "toml"
	JSON       ID = "json"
	Bash       ID = "bash"
	C          ID = "c"
	CPP        ID = "cpp"
)

var extMap = map[string]ID{
	".go":    Go,
	".rs":    Rust,
	".py":    Python,
	".js":    JavaScript,
	".jsx":   JavaScript,
	".mjs":   JavaScript,
	".cjs":   JavaScript,
	".ts":    TypeScript,
	".tsx":   TSX,
	".yaml":  YAML,
	".yml":   YAML,
	".toml":  TOML,
	".json":  JSON,
	".jsonc": JSON,
	".json5": JSON,
	".sh":    Bash,
	".bash":  Bash,
	".zsh":   Bash,
	".c":     C,
	".h":     C,
	".cpp":   CPP,
	".cc":    CPP,
	".cxx":   CPP,
	".hpp":   CPP,
	".hh":    CPP,

	".java":  Plain,
	".kt":    Plain,
	".swift": Plain,
	".rb":    Plain,
	".php":   Plain,
	".lua":   Plain,
	".ini":   Plain,
	".conf":  Plain,
	".md":    Plain,
}

var fileMap = map[string]ID{
	"Makefile":          Plain,
	"Dockerfile":        Plain,
	".bashrc":           Bash,
	".zshrc":            Bash,
	".gitignore":        Plain,
	".editorconfig":     Plain,
	"Cargo.toml":        TOML,
	"package-lock.json": JSON,
	"go.mod":            Go,
	"go.sum":            Plain,
}

func Detect(path string) ID {
	base := filepath.Base(path)
	if id, ok := fileMap[base]; ok {
		return id
	}
	ext := strings.ToLower(filepath.Ext(base))
	if id, ok := extMap[ext]; ok {
		return id
	}
	return Plain
}

func DetectWithShebang(path string, firstLine string) ID {
	if id := Detect(path); id != Plain {
		return id
	}

	if !strings.HasPrefix(firstLine, "#!") {
		return Plain
	}
	lower := strings.ToLower(firstLine)
	switch {
	case strings.Contains(lower, "python"):
		return Python
	case strings.Contains(lower, "bash") || strings.Contains(lower, "zsh") || strings.Contains(lower, "sh"):
		return Bash
	case strings.Contains(lower, "node"):
		return JavaScript
	default:
		return Plain
	}
}
