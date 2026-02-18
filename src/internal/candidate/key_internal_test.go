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
