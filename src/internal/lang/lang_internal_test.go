package lang

import "testing"

func TestDetectSwiftByExtension(t *testing.T) {
	if got := Detect("App/Core/Service.swift"); got != Swift {
		t.Fatalf("Detect(.swift) = %q, want %q", got, Swift)
	}
}
