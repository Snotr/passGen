package generator

import (
	"testing"

	"github.com/Snotr/passGen/internal/parser"
)

var testGroups = []parser.TagGroup{
	{Tag: "names", Values: []string{"alice", "bob"}},
	{Tag: "numbers", Values: []string{"1", "2"}},
	{Tag: "symbols", Values: []string{"!", "@"}},
}

func TestParsePatternValid(t *testing.T) {
	tags, err := ParsePattern("{names}{numbers}{symbols}", testGroups)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"names", "numbers", "symbols"}
	if len(tags) != len(want) {
		t.Fatalf("got %d tags, want %d", len(tags), len(want))
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, want[i])
		}
	}
}

func TestParsePatternSubset(t *testing.T) {
	tags, err := ParsePattern("{names}{symbols}", testGroups)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(tags))
	}
}

func TestParsePatternEmpty(t *testing.T) {
	tags, err := ParsePattern("", testGroups)
	if err != nil {
		t.Fatal(err)
	}
	if tags != nil {
		t.Errorf("expected nil for empty pattern, got %v", tags)
	}
}

func TestParsePatternUnknownTag(t *testing.T) {
	_, err := ParsePattern("{unknown}", testGroups)
	if err == nil {
		t.Error("expected error for unknown tag")
	}
}

func TestParsePatternUnclosedBrace(t *testing.T) {
	_, err := ParsePattern("{names", testGroups)
	if err == nil {
		t.Error("expected error for unclosed brace")
	}
}

func TestParsePatternTextOutsideBraces(t *testing.T) {
	_, err := ParsePattern("prefix{names}", testGroups)
	if err == nil {
		t.Error("expected error for text outside braces")
	}
}

func TestParsePatternEmptyTag(t *testing.T) {
	_, err := ParsePattern("{}", testGroups)
	if err == nil {
		t.Error("expected error for empty tag name")
	}
}

func TestParsePatternCaseInsensitive(t *testing.T) {
	tags, err := ParsePattern("{Names}{NUMBERS}", testGroups)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0] != "names" || tags[1] != "numbers" {
		t.Errorf("expected case-insensitive match, got %v", tags)
	}
}
