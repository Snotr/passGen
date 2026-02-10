// Package tui provides an interactive terminal controller for monitoring and
// controlling password generation with pause, resume, and stop support.
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

// State represents the TUI controller state.
type State int32

const (
	StateRunning State = iota
	StatePaused
	StateStopped
)

// GeneratorControl is the interface the TUI uses to control the generator.
type GeneratorControl interface {
	Pause()
	Resume()
	Stop()
}

// ProgressReader provides generation progress metrics.
type ProgressReader interface {
	Generated() int64
	Total() int64
	Elapsed() time.Duration
}

// Controller manages interactive terminal state.
type Controller struct {
	gen      GeneratorControl
	progress ProgressReader
	stdin    io.Reader
	stderr   io.Writer
	state    atomic.Int32
	done     chan struct{}
	finished chan struct{} // closed when Run returns
	keyCh    chan byte

	// SavePromptFunc is called when the user presses [s].
	// It runs while the terminal is still in raw mode, so newlines in
	// output must be written as \r\n and single-byte reads return
	// immediately without requiring Enter.
	// It should return true if the user wants to save partial results.
	SavePromptFunc func() bool

	// SaveDeclined is set to true when the user answers "no" to the save prompt.
	SaveDeclined bool
}

// NewController creates a new TUI controller.
func NewController(gen GeneratorControl, progress ProgressReader) *Controller {
	return &Controller{
		gen:      gen,
		progress: progress,
		stdin:    os.Stdin,
		stderr:   os.Stderr,
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}
}

// Run starts the interactive TUI. It puts stdin into raw mode,
// listens for keypresses, and renders progress to stderr.
// It blocks until the done channel is closed or the user stops generation.
// Terminal state is always restored on return.
func (c *Controller) Run(ctx context.Context) error {
	defer close(c.finished)

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Not a terminal; just run progress without interactive controls
		return c.runNonInteractive(ctx)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return c.runNonInteractive(ctx)
	}
	defer term.Restore(fd, oldState)

	// Keypress reader goroutine
	c.keyCh = make(chan byte, 8)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			select {
			case c.keyCh <- buf[0]:
			case <-c.done:
				return
			}
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	c.state.Store(int32(StateRunning))
	c.renderProgress()

	for {
		select {
		case <-ctx.Done():
			c.clearLine()
			return nil

		case <-c.done:
			c.renderFinalProgress()
			return nil

		case key := <-c.keyCh:
			switch key {
			case 'p', 'P':
				c.togglePause()
			case 's', 'S':
				c.handleStop()
				return nil
			case 'q', 'Q', 3: // q/Ctrl+C: quit immediately without saving
				c.gen.Stop()
				c.state.Store(int32(StateStopped))
				c.clearLine()
				c.SaveDeclined = true
				return nil
			}

		case <-ticker.C:
			c.renderProgress()
		}
	}
}

func (c *Controller) runNonInteractive(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.done:
			c.renderFinalProgressNewline()
			return nil
		case <-ticker.C:
			c.renderProgressNewline()
		}
	}
}

// Done signals the TUI that generation is complete.
func (c *Controller) Done() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

// Wait blocks until Run has fully returned (terminal restored, final output rendered).
func (c *Controller) Wait() {
	<-c.finished
}

func (c *Controller) togglePause() {
	cur := State(c.state.Load())
	if cur == StateRunning {
		c.gen.Pause()
		c.state.Store(int32(StatePaused))
	} else if cur == StatePaused {
		c.gen.Resume()
		c.state.Store(int32(StateRunning))
	}
	c.renderProgress()
}

func (c *Controller) handleStop() {
	c.gen.Stop()
	c.state.Store(int32(StateStopped))
	c.clearLine()

	if c.SavePromptFunc != nil {
		// Stay in raw mode so the prompt can read a single keypress
		// without requiring Enter. The deferred term.Restore in Run
		// will restore the terminal when we return.
		saved := c.SavePromptFunc()
		c.SaveDeclined = !saved
	}
}

// ReadKey reads a single keypress from the internal key channel.
// It should only be called from SavePromptFunc during handleStop.
func (c *Controller) ReadKey() (byte, bool) {
	key, ok := <-c.keyCh
	return key, ok
}

func (c *Controller) renderProgress() {
	generated := c.progress.Generated()
	total := c.progress.Total()
	elapsed := c.progress.Elapsed().Truncate(time.Millisecond)
	state := State(c.state.Load())

	var stateStr string
	switch state {
	case StatePaused:
		stateStr = " PAUSED |"
	default:
		stateStr = ""
	}

	var speedStr string
	if secs := c.progress.Elapsed().Seconds(); secs > 0 {
		speed := float64(generated) / secs
		speedStr = fmt.Sprintf(" | Speed: %s/s", fmtNum(int64(speed)))
	}

	if total > 0 {
		pct := float64(generated) / float64(total) * 100
		fmt.Fprintf(c.stderr, "\r\033[K  %s Generated: %s / %s (%.1f%%) | Elapsed: %v%s | [p]ause [s]top [q]uit",
			stateStr, fmtNum(generated), fmtNum(total), pct, elapsed, speedStr)
	} else {
		fmt.Fprintf(c.stderr, "\r\033[K  %s Generated: %s | Elapsed: %v%s | [p]ause [s]top [q]uit",
			stateStr, fmtNum(generated), elapsed, speedStr)
	}
}

func (c *Controller) renderFinalProgress() {
	generated := c.progress.Generated()
	elapsed := c.progress.Elapsed().Truncate(time.Millisecond)
	fmt.Fprintf(c.stderr, "\r\033[K  Done. Generated %s passwords in %v.\r\n\n", fmtNum(generated), elapsed)
}

func (c *Controller) renderProgressNewline() {
	generated := c.progress.Generated()
	elapsed := c.progress.Elapsed().Truncate(time.Millisecond)
	fmt.Fprintf(c.stderr, "  Generated: %s | Elapsed: %v\n", fmtNum(generated), elapsed)
}

func (c *Controller) renderFinalProgressNewline() {
	generated := c.progress.Generated()
	elapsed := c.progress.Elapsed().Truncate(time.Millisecond)
	fmt.Fprintf(c.stderr, "  Done. Generated %s passwords in %v.\n\n", fmtNum(generated), elapsed)
}

func (c *Controller) clearLine() {
	fmt.Fprintf(c.stderr, "\r\033[K")
}

// fmtNum formats an integer with comma separators.
func fmtNum(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	rem := len(s) % 3
	if rem > 0 {
		result = append(result, s[:rem]...)
	}
	for i := rem; i < len(s); i += 3 {
		if len(result) > 0 {
			result = append(result, ',')
		}
		result = append(result, s[i:i+3]...)
	}
	return string(result)
}
