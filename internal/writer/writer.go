// Package writer provides TXT and CSV output writers for generated passwords.
package writer

import (
	"bufio"
	"encoding/csv"
	"io"
	"sync"
)

// Writer defines the output interface for writing generated passwords.
type Writer interface {
	WritePassword(password string) error
	Flush() error
	Close() error
}

// TXTWriter writes one password per line.
type TXTWriter struct {
	mu  sync.Mutex
	bw  *bufio.Writer
	out io.Writer
}

// NewTXTWriter returns a Writer that outputs one password per line.
func NewTXTWriter(w io.Writer) *TXTWriter {
	return &TXTWriter{
		bw:  bufio.NewWriter(w),
		out: w,
	}
}

func (w *TXTWriter) WritePassword(password string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.bw.WriteString(password + "\n")
	return err
}

func (w *TXTWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bw.Flush()
}

func (w *TXTWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}
	if c, ok := w.out.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// CSVWriter writes passwords as CSV rows.
type CSVWriter struct {
	mu      sync.Mutex
	cw      *csv.Writer
	out     io.Writer
	headers []string
	wrote   bool
}

// NewCSVWriter returns a Writer that outputs passwords in CSV format.
// If headers is non-empty, a header row is written before the first password.
func NewCSVWriter(w io.Writer, headers []string) *CSVWriter {
	return &CSVWriter{
		cw:      csv.NewWriter(w),
		out:     w,
		headers: headers,
	}
}

func (w *CSVWriter) WritePassword(password string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.wrote && len(w.headers) > 0 {
		if err := w.cw.Write(w.headers); err != nil {
			return err
		}
		w.wrote = true
	}
	return w.cw.Write([]string{password})
}

func (w *CSVWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cw.Flush()
	return w.cw.Error()
}

func (w *CSVWriter) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}
	if c, ok := w.out.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
