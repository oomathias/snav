package candidate

import "snav/internal/lang"

const DefaultRGPattern = `^(?:\s*(?:(?:export|default|async|public|private|protected|internal|abstract|final|sealed|partial|static|inline|open|override|readonly|extern|unsafe|suspend|data|pub(?:\([^)]*\))?)\s+)*(?:func|function|type|typealias|var|const|class|interface|enum|record|def|fn|fun|struct|impl|trait|module|mod|let|object|protocol|extension|namespace|test)\b|\s*(?:(?:public|private|protected|internal|static|final|abstract|virtual|override|async|extern|unsafe|sealed|partial|readonly|synchronized|native|strictfp)\s+)+(?:[A-Za-z_][A-Za-z0-9_<>,.?\[\]]*\s+)+[A-Za-z_][A-Za-z0-9_]*\s*\()`
const DefaultRGConfigPattern = `^\s*(?:\[\[[A-Za-z0-9_.:-]+\]\]\s*$|\[[A-Za-z0-9_.:-]+\]\s*$|"(?:\\.|[^"\\])+"\s*:|'[^']+'\s*:|-\s*(?:"(?:\\.|[^"\\])+"|'[^']+'|[A-Za-z0-9_.-]+)\s*:|(?:export\s+)?[A-Za-z0-9_.-]+\s*(?::|=)|[A-Za-z0-9_.-]+(?:\s+"(?:\\.|[^"\\])+"){0,2}\s*\{|<\s*[A-Za-z_][A-Za-z0-9_.:-]*(?:\s|>|/>))`

type LangID = lang.ID

const (
	LangPlain      LangID = lang.Plain
	LangGo         LangID = lang.Go
	LangRust       LangID = lang.Rust
	LangZig        LangID = lang.Zig
	LangCSharp     LangID = lang.CSharp
	LangJava       LangID = lang.Java
	LangKotlin     LangID = lang.Kotlin
	LangPHP        LangID = lang.PHP
	LangRuby       LangID = lang.Ruby
	LangPython     LangID = lang.Python
	LangJavaScript LangID = lang.JavaScript
	LangTypeScript LangID = lang.TypeScript
	LangTSX        LangID = lang.TSX
	LangSwift      LangID = lang.Swift
	LangYAML       LangID = lang.YAML
	LangTOML       LangID = lang.TOML
	LangJSON       LangID = lang.JSON
	LangBash       LangID = lang.Bash
	LangC          LangID = lang.C
	LangCPP        LangID = lang.CPP
)

type Candidate struct {
	ID            int
	File          string
	Line          int
	Col           int
	Text          string
	Key           string
	LangID        LangID
	SemanticScore int16
}

type ProducerConfig struct {
	Root         string
	Pattern      string
	Excludes     []string
	NoIgnore     bool
	ExcludeTests bool
}

type FilteredCandidate struct {
	Index    int32
	Score    int32
	OpenLine int32
	OpenCol  int32
}

var filterParallelThreshold = 20_000
var filterMinChunkSize = 4_096

var testExcludeGlobs = []string{
	"test/**",
	"tests/**",
	"__tests__/**",
	"spec/**",
	"specs/**",
	"**/test/**",
	"**/tests/**",
	"**/__tests__/**",
	"**/spec/**",
	"**/specs/**",
	"*_test.*",
	"*_spec.*",
	"*.test.*",
	"*.spec.*",
	"test_*.py",
	"**/*_test.*",
	"**/*_spec.*",
	"**/*.test.*",
	"**/*.spec.*",
	"**/test_*.py",
}

var configIncludeGlobs = []string{
	"*.json",
	"*.jsonc",
	"*.json5",
	"*.yaml",
	"*.yml",
	"*.toml",
	"*.ini",
	".env",
	".env.*",
	".envrc",
	"*.properties",
	"*.conf",
	"*.cfg",
	"*.cnf",
	"*.tf",
	"*.hcl",
	"*.tfvars",
	"*.xml",
	"*.plist",
	"*.csproj",
	"*.props",
	"*.targets",
	"*.config",
}

var declarationIncludeGlobs = []string{
	"*.c",
	"*.cc",
	"*.cpp",
	"*.cxx",
	"*.h",
	"*.hh",
	"*.hpp",
	"*.hxx",
	"*.inc",
	"*.def",
	"*.td",
	"*.go",
	"*.rs",
	"*.zig",
	"build.zig",
	"*.py",
	"*.js",
	"*.jsx",
	"*.mjs",
	"*.cjs",
	"*.ts",
	"*.tsx",
	"*.cs",
	"*.csx",
	"*.java",
	"*.kt",
	"*.kts",
	"*.swift",
	"*.php",
	"*.php4",
	"*.php5",
	"*.phtml",
	"*.rb",
	"Gemfile",
	"Rakefile",
	".irbrc",
	"*.sh",
	"*.bash",
	"*.zsh",
	".bashrc",
	".zshrc",
}
