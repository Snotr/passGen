package tui

import (
	"sync/atomic"
	"testing"
	"time"
)

type mockGenerator struct {
	paused  atomic.Bool
	stopped atomic.Bool
}

func (m *mockGenerator) Pause()  { m.paused.Store(true) }
func (m *mockGenerator) Resume() { m.paused.Store(false) }
func (m *mockGenerator) Stop()   { m.stopped.Store(true) }

type mockProgress struct {
	generated atomic.Int64
	total     int64
	start     time.Time
}

func (m *mockProgress) Generated() int64       { return m.generated.Load() }
func (m *mockProgress) Total() int64           { return m.total }
func (m *mockProgress) Elapsed() time.Duration { return time.Since(m.start) }

func TestControllerStateTransitions(t *testing.T) {
	gen := &mockGenerator{}
	prog := &mockProgress{total: 100, start: time.Now()}
	ctrl := NewController(gen, prog)

	// Initial state is running (after store)
	ctrl.state.Store(int32(StateRunning))

	// Pause
	ctrl.togglePause()
	if State(ctrl.state.Load()) != StatePaused {
		t.Error("expected StatePaused after togglePause from running")
	}
	if !gen.paused.Load() {
		t.Error("expected generator to be paused")
	}

	// Resume
	ctrl.togglePause()
	if State(ctrl.state.Load()) != StateRunning {
		t.Error("expected StateRunning after togglePause from paused")
	}
	if gen.paused.Load() {
		t.Error("expected generator to be resumed")
	}
}

func TestControllerDone(t *testing.T) {
	gen := &mockGenerator{}
	prog := &mockProgress{start: time.Now()}
	ctrl := NewController(gen, prog)

	// Done should be idempotent
	ctrl.Done()
	ctrl.Done() // should not panic

	select {
	case <-ctrl.done:
		// Good, channel is closed
	default:
		t.Error("done channel should be closed")
	}
}

func TestStateConstants(t *testing.T) {
	if StateRunning != 0 {
		t.Errorf("StateRunning = %d, want 0", StateRunning)
	}
	if StatePaused != 1 {
		t.Errorf("StatePaused = %d, want 1", StatePaused)
	}
	if StateStopped != 2 {
		t.Errorf("StateStopped = %d, want 2", StateStopped)
	}
}
