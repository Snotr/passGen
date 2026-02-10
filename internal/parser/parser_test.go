package parser

import (
	"testing"
)

func TestClassifyValueNumbers(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"123", "numbers"},
		{"0", "numbers"},
		{"999999", "numbers"},
	}
	for _, tt := range tests {
		if got := ClassifyValue(tt.input); got != tt.want {
			t.Errorf("ClassifyValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifyValueSymbols(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"!", "symbols"},
		{"@#$", "symbols"},
		{"!@", "symbols"},
	}
	for _, tt := range tests {
		if got := ClassifyValue(tt.input); got != tt.want {
			t.Errorf("ClassifyValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifyValueWords(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "words"},
		{"abc123", "words"},
		{"hello!", "words"},
		{"", "words"},
	}
	for _, tt := range tests {
		if got := ClassifyValue(tt.input); got != tt.want {
			t.Errorf("ClassifyValue(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMergeGroupsSameTag(t *testing.T) {
	groups := []TagGroup{
		{Tag: "words", Values: []string{"hello", "world"}},
		{Tag: "words", Values: []string{"world", "foo"}},
	}
	merged := MergeGroups(groups)
	if len(merged) != 1 {
		t.Fatalf("expected 1 group, got %d", len(merged))
	}
	if merged[0].Tag != "words" {
		t.Errorf("expected tag 'words', got %q", merged[0].Tag)
	}
	want := []string{"hello", "world", "foo"}
	if len(merged[0].Values) != len(want) {
		t.Fatalf("expected %d values, got %d: %v", len(want), len(merged[0].Values), merged[0].Values)
	}
	for i, v := range want {
		if merged[0].Values[i] != v {
			t.Errorf("value[%d] = %q, want %q", i, merged[0].Values[i], v)
		}
	}
}

func TestMergeGroupsMultipleTags(t *testing.T) {
	groups := []TagGroup{
		{Tag: "names", Values: []string{"alice"}},
		{Tag: "numbers", Values: []string{"1"}},
		{Tag: "names", Values: []string{"bob"}},
	}
	merged := MergeGroups(groups)
	if len(merged) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(merged))
	}
	if merged[0].Tag != "names" {
		t.Errorf("first group tag = %q, want 'names'", merged[0].Tag)
	}
	if len(merged[0].Values) != 2 {
		t.Errorf("expected 2 name values, got %d", len(merged[0].Values))
	}
	if merged[1].Tag != "numbers" {
		t.Errorf("second group tag = %q, want 'numbers'", merged[1].Tag)
	}
}

func TestMergeGroupsEmpty(t *testing.T) {
	merged := MergeGroups(nil)
	if len(merged) != 0 {
		t.Errorf("expected 0 groups, got %d", len(merged))
	}
}

func TestMergeGroupsDedup(t *testing.T) {
	groups := []TagGroup{
		{Tag: "x", Values: []string{"a", "a", "b"}},
		{Tag: "x", Values: []string{"b", "c"}},
	}
	merged := MergeGroups(groups)
	if len(merged) != 1 {
		t.Fatalf("expected 1 group, got %d", len(merged))
	}
	want := []string{"a", "b", "c"}
	if len(merged[0].Values) != len(want) {
		t.Fatalf("expected %d values, got %d: %v", len(want), len(merged[0].Values), merged[0].Values)
	}
	for i, v := range want {
		if merged[0].Values[i] != v {
			t.Errorf("value[%d] = %q, want %q", i, merged[0].Values[i], v)
		}
	}
}
