package parser

import (
	"testing"
)

func TestParseCSVSimple(t *testing.T) {
	groups, err := ParseCSV(testdataPath("simple.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %+v", len(groups), groups)
	}

	expected := []struct {
		tag    string
		count  int
		values []string
	}{
		{"names", 2, []string{"alice", "bob"}},
		{"numbers", 2, []string{"1", "2"}},
		{"symbols", 2, []string{"!", "@"}},
	}
	for i, e := range expected {
		if groups[i].Tag != e.tag {
			t.Errorf("group[%d].Tag = %q, want %q", i, groups[i].Tag, e.tag)
		}
		if len(groups[i].Values) != e.count {
			t.Errorf("group[%d] (%s) values: got %d, want %d", i, e.tag, len(groups[i].Values), e.count)
		}
		for j, v := range e.values {
			if j < len(groups[i].Values) && groups[i].Values[j] != v {
				t.Errorf("group[%d].Values[%d] = %q, want %q", i, j, groups[i].Values[j], v)
			}
		}
	}
}

func TestParseCSVMalformed(t *testing.T) {
	// malformed.csv has ragged rows; should parse without error
	groups, err := ParseCSV(testdataPath("malformed.csv"))
	if err != nil {
		t.Fatal(err)
	}

	// Should have at least the names group with "alice" and "bob"
	if len(groups) == 0 {
		t.Fatal("expected at least 1 group")
	}

	found := false
	for _, g := range groups {
		if g.Tag == "names" {
			found = true
			if len(g.Values) != 2 {
				t.Errorf("expected 2 values in names group, got %d: %v", len(g.Values), g.Values)
			}
		}
	}
	if !found {
		t.Errorf("expected 'names' group in: %+v", groups)
	}
}

func TestParseCSVNonexistentFile(t *testing.T) {
	_, err := ParseCSV(testdataPath("nonexistent.csv"))
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
