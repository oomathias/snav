package main

import (
	"regexp"
	"testing"
)

func TestFilterCandidatesPrefersMatchingCase(t *testing.T) {
	candidates := []Candidate{
		{ID: 1, File: "a.go", Text: "func myFunc() {}", Key: "myFunc"},
		{ID: 2, File: "b.go", Text: "func MyFunc() {}", Key: "MyFunc"},
	}

	res := filterCandidates(candidates, "MyF")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[res[0].Index].Key; got != "MyFunc" {
		t.Fatalf("expected MyFunc first for mixed-case query, got %s", got)
	}

	res = filterCandidates(candidates, "myf")
	if len(res) < 2 {
		t.Fatalf("expected at least 2 matches, got %d", len(res))
	}
	if got := candidates[res[0].Index].Key; got != "myFunc" {
		t.Fatalf("expected myFunc first for lowercase query, got %s", got)
	}
}

func TestDefaultRGPatternNamespaceAndClasses(t *testing.T) {
	re := regexp.MustCompile(defaultRGPattern)

	matchCases := []string{
		"namespace Symfind.Core;",
		"inline namespace v1 {",
		"public class SearchIndex : Base {",
		"export default class QueryEngine {",
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
