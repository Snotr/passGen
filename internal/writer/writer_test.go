package writer

import (
	"bytes"
	"strings"
	"testing"
)

func TestTXTWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewTXTWriter(&buf)

	passwords := []string{"hello123!", "world456@", "test789#"}
	for _, pw := range passwords {
		if err := w.WritePassword(pw); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}
	for i, pw := range passwords {
		if lines[i] != pw {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], pw)
		}
	}
}

func TestTXTWriterEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := NewTXTWriter(&buf)
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestCSVWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewCSVWriter(&buf, []string{"password"})

	passwords := []string{"hello123!", "world456@"}
	for _, pw := range passwords {
		if err := w.WritePassword(pw); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("expected 3 lines (header + 2 rows), got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "password" {
		t.Errorf("header = %q, want 'password'", lines[0])
	}
	if lines[1] != "hello123!" {
		t.Errorf("row[0] = %q, want 'hello123!'", lines[1])
	}
}

func TestCSVWriterNoHeaders(t *testing.T) {
	var buf bytes.Buffer
	w := NewCSVWriter(&buf, nil)

	if err := w.WritePassword("test"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %q", len(lines), buf.String())
	}
	if lines[0] != "test" {
		t.Errorf("row = %q, want 'test'", lines[0])
	}
}

func TestWriterInterface(t *testing.T) {
	// Verify both types implement Writer
	var _ Writer = (*TXTWriter)(nil)
	var _ Writer = (*CSVWriter)(nil)
}
