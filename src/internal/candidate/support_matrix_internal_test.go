package candidate

import (
	"path/filepath"
	"testing"

	"snav/internal/lang"
)

func TestDeclarationIncludeGlobsCoverSupportedSourceFiles(t *testing.T) {
	tests := []struct {
		path string
		want lang.ID
	}{
		{path: "main.go", want: lang.Go},
		{path: "lib.rs", want: lang.Rust},
		{path: "build.zig", want: lang.Zig},
		{path: "Program.cs", want: lang.CSharp},
		{path: "Script.csx", want: lang.CSharp},
		{path: "Main.java", want: lang.Java},
		{path: "App.kt", want: lang.Kotlin},
		{path: "Script.kts", want: lang.Kotlin},
		{path: "index.php", want: lang.PHP},
		{path: "index.php4", want: lang.PHP},
		{path: "index.php5", want: lang.PHP},
		{path: "index.phtml", want: lang.PHP},
		{path: "Gemfile", want: lang.Ruby},
		{path: "Rakefile", want: lang.Ruby},
		{path: ".irbrc", want: lang.Ruby},
		{path: "app.rb", want: lang.Ruby},
		{path: "main.py", want: lang.Python},
		{path: "main.js", want: lang.JavaScript},
		{path: "main.jsx", want: lang.JavaScript},
		{path: "main.mjs", want: lang.JavaScript},
		{path: "main.cjs", want: lang.JavaScript},
		{path: "main.ts", want: lang.TypeScript},
		{path: "main.tsx", want: lang.TSX},
		{path: "Service.swift", want: lang.Swift},
		{path: ".bashrc", want: lang.Bash},
		{path: ".zshrc", want: lang.Bash},
		{path: "run.sh", want: lang.Bash},
		{path: "run.bash", want: lang.Bash},
		{path: "run.zsh", want: lang.Bash},
		{path: "main.c", want: lang.C},
		{path: "main.h", want: lang.C},
		{path: "main.cpp", want: lang.CPP},
		{path: "main.cc", want: lang.CPP},
		{path: "main.cxx", want: lang.CPP},
		{path: "main.hpp", want: lang.CPP},
		{path: "main.hh", want: lang.CPP},
		{path: "main.hxx", want: lang.CPP},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := lang.Detect(tt.path); got != tt.want {
				t.Fatalf("lang.Detect(%q) = %q, want %q", tt.path, got, tt.want)
			}
			if !matchesAnyGlob(declarationIncludeGlobs, filepath.Base(tt.path)) {
				t.Fatalf("declarationIncludeGlobs do not cover %q", tt.path)
			}
		})
	}
}

func TestConfigIncludeGlobsCoverPreviewConfigFiles(t *testing.T) {
	tests := []struct {
		path string
		want lang.ID
	}{
		{path: "config.json", want: lang.JSON},
		{path: "config.jsonc", want: lang.JSON},
		{path: "config.json5", want: lang.JSON},
		{path: "config.yaml", want: lang.YAML},
		{path: "config.yml", want: lang.YAML},
		{path: "config.toml", want: lang.TOML},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := lang.Detect(tt.path); got != tt.want {
				t.Fatalf("lang.Detect(%q) = %q, want %q", tt.path, got, tt.want)
			}
			if !matchesAnyGlob(configIncludeGlobs, filepath.Base(tt.path)) {
				t.Fatalf("configIncludeGlobs do not cover %q", tt.path)
			}
		})
	}
}

func matchesAnyGlob(globs []string, name string) bool {
	for _, glob := range globs {
		ok, err := filepath.Match(glob, name)
		if err != nil {
			continue
		}
		if ok {
			return true
		}
	}
	return false
}
