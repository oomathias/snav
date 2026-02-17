package main

import (
	"reflect"
	"testing"
)

func TestProjectSpansToDisplayTabExpansion(t *testing.T) {
	base := []Span{
		{Start: 0, End: 1, Cat: TokenPlain},
		{Start: 1, End: 7, Cat: TokenKeyword},
		{Start: 7, End: 13, Cat: TokenPlain},
	}

	got, ok := projectSpansToDisplay(base, "\treturn value", "    return value")
	if !ok {
		t.Fatalf("expected projection to succeed")
	}

	want := []Span{
		{Start: 0, End: 4, Cat: TokenPlain},
		{Start: 4, End: 10, Cat: TokenKeyword},
		{Start: 10, End: 16, Cat: TokenPlain},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans = %#v, want %#v", got, want)
	}
}

func TestProjectSpansToDisplayTabExpansionWithEllipsis(t *testing.T) {
	base := []Span{
		{Start: 0, End: 1, Cat: TokenPlain},
		{Start: 1, End: 7, Cat: TokenKeyword},
		{Start: 7, End: 13, Cat: TokenPlain},
	}

	got, ok := projectSpansToDisplay(base, "\treturn value", "    return...")
	if !ok {
		t.Fatalf("expected projection to succeed")
	}

	want := []Span{
		{Start: 0, End: 4, Cat: TokenPlain},
		{Start: 4, End: 10, Cat: TokenKeyword},
		{Start: 10, End: 13, Cat: TokenPlain},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spans = %#v, want %#v", got, want)
	}
}
