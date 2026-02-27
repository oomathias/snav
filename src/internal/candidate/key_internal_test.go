package candidate

import (
	"regexp"
	"testing"
)

func TestDefaultRGPatternNamespaceAndClasses(t *testing.T) {
	re := regexp.MustCompile(DefaultRGPattern)

	matchCases := []string{
		"namespace Symfind.Core;",
		"inline namespace v1 {",
		"public class SearchIndex : Base {",
		"export default class QueryEngine {",
		"pub struct GitPanel {",
		"pub(crate) struct GitPanel {",
	}

	for _, tc := range matchCases {
		if !re.MatchString(tc) {
			t.Fatalf("pattern should match %q", tc)
		}
	}

	nonMatchCases := []string{
		"using namespace std;",
		"return className;",
	}

	for _, tc := range nonMatchCases {
		if re.MatchString(tc) {
			t.Fatalf("pattern should not match %q", tc)
		}
	}
}

func TestDefaultRGConfigPatternMatchesConfigEntries(t *testing.T) {
	re := regexp.MustCompile(DefaultRGConfigPattern)

	matchCases := []string{
		`"name": "snav",`,
		"root: ./src",
		"root = ./src",
		"export APP_ENV=dev",
		"- name: web",
		"[tool.mise]",
		"[[plugins]]",
		`resource "aws_instance" "web" {`,
		`terraform {`,
		`<appSettings>`,
		`<add key="ConnectionStrings__Main" value="dsn" />`,
	}

	for _, tc := range matchCases {
		if !re.MatchString(tc) {
			t.Fatalf("config pattern should match %q", tc)
		}
	}

	nonMatchCases := []string{
		"# comment",
		"{",
		"func main() {",
		"resource aws_instance web {",
		"- just text",
		"</appSettings>",
		"return value",
	}

	for _, tc := range nonMatchCases {
		if re.MatchString(tc) {
			t.Fatalf("config pattern should not match %q", tc)
		}
	}
}

func TestExtractKeyNamespaceAndClasses(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "dot namespace", text: "namespace Symfind.Core;", want: "Symfind.Core"},
		{name: "cpp namespace", text: "inline namespace symfind::core {", want: "symfind::core"},
		{name: "csharp class", text: "public class SearchIndex : Base {", want: "SearchIndex"},
		{name: "default export class", text: "export default class QueryEngine {", want: "QueryEngine"},
		{name: "final class", text: "final class Tokenizer extends Base {}", want: "Tokenizer"},
		{name: "rust pub struct", text: "pub struct GitPanel {", want: "GitPanel"},
		{name: "rust scoped pub struct", text: "pub(crate) struct GitPanel {", want: "GitPanel"},
		{name: "json key", text: `"editor.fontSize": 14,`, want: "editor.fontSize"},
		{name: "yaml key", text: "log-level: debug", want: "log-level"},
		{name: "toml key", text: "log.level = \"debug\"", want: "log.level"},
		{name: "yaml list key", text: "- name: web", want: "name"},
		{name: "ini section", text: "[database.production]", want: "database.production"},
		{name: "toml array section", text: "[[inputs.http]]", want: "inputs.http"},
		{name: "dotenv export", text: "export APP_ENV=dev", want: "APP_ENV"},
		{name: "properties key", text: "log.level=debug", want: "log.level"},
		{name: "hcl resource block label", text: `resource "aws_instance" "web" {`, want: "web"},
		{name: "hcl module block label", text: `module "network" {`, want: "network"},
		{name: "hcl simple block", text: "terraform {", want: "terraform"},
		{name: "xml key attr", text: `<add key="ConnectionStrings__Main" value="dsn" />`, want: "ConnectionStrings__Main"},
		{name: "xml section tag", text: `<appSettings>`, want: "appSettings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractKey(tt.text, "src/sample.txt")
			if got != tt.want {
				t.Fatalf("ExtractKey(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
