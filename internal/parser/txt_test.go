package parser

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func TestParseTXTSimple(t *testing.T) {
	groups, err := ParseTXT(testdataPath("simple.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %+v", len(groups), groups)
	}

	if groups[0].Tag != "words" {
		t.Errorf("group[0].Tag = %q, want 'words'", groups[0].Tag)
	}
	if len(groups[0].Values) != 2 {
		t.Errorf("group[0] values: got %d, want 2", len(groups[0].Values))
	}

	if groups[1].Tag != "numbers" {
		t.Errorf("group[1].Tag = %q, want 'numbers'", groups[1].Tag)
	}
	if len(groups[1].Values) != 2 {
		t.Errorf("group[1] values: got %d, want 2", len(groups[1].Values))
	}
}

func TestParseTXTMultiTag(t *testing.T) {
	groups, err := ParseTXT(testdataPath("multi_tag.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %+v", len(groups), groups)
	}

	expected := []struct {
		tag   string
		count int
	}{
		{"names", 2},
		{"symbols", 2},
		{"numbers", 2},
	}
	for i, e := range expected {
		if groups[i].Tag != e.tag {
			t.Errorf("group[%d].Tag = %q, want %q", i, groups[i].Tag, e.tag)
		}
		if len(groups[i].Values) != e.count {
			t.Errorf("group[%d] (%s) values: got %d, want %d", i, e.tag, len(groups[i].Values), e.count)
		}
	}
}

func TestParseTXTDefaultTag(t *testing.T) {
	groups, err := ParseTXT(testdataPath("malformed.txt"))
	if err != nil {
		t.Fatal(err)
	}

	// "no tag here" and "some data" should be classified as "words"
	// Then #words tag with "valid"
	// The heuristic classifier should put untagged text into "words"
	found := false
	for _, g := range groups {
		if g.Tag == "words" {
			found = true
			if len(g.Values) < 1 {
				t.Errorf("expected at least 1 value in 'words' group, got %d", len(g.Values))
			}
		}
	}
	if !found {
		t.Errorf("expected a 'words' group, got groups: %+v", groups)
	}
}

func TestParseTXTEmptyLines(t *testing.T) {
	// malformed.txt has blank lines; they should be skipped
	groups, err := ParseTXT(testdataPath("malformed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		for _, v := range g.Values {
			if v == "" {
				t.Errorf("empty value found in group %q", g.Tag)
			}
		}
	}
}

func TestParseTXTNonexistentFile(t *testing.T) {
	_, err := ParseTXT(testdataPath("nonexistent.txt"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
