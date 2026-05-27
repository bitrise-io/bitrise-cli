package build

import (
	"testing"

	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
)

// The TUI used to print each log line with tea.Batch, which bubbletea runs
// concurrently — so multi-line chunks and back-to-back chunks scrambled on
// screen even though Watch delivered them in order. These tests pin the
// single-flight serialization that replaced it: at most one print is in
// flight, and lines arriving during a print buffer until it completes.

func TestWaitModel_SerializesLogOutputSingleFlight(t *testing.T) {
	m := newWaitModel(internalbuild.Build{BuildNumber: 1})

	// First chunk with two complete lines starts exactly one print and
	// drains the lines into that in-flight block.
	next, cmd := m.Update(logChunkMsg("alpha\nbeta\n"))
	wm := next.(waitModel)
	if !wm.printing {
		t.Fatal("expected printing=true after first chunk")
	}
	if len(wm.pending) != 0 {
		t.Fatalf("expected pending drained into print block, got %v", wm.pending)
	}
	if cmd == nil {
		t.Fatal("expected a print command")
	}

	// A second chunk arriving while the print is in flight must buffer in
	// pending and must NOT issue a second, racing print command.
	next, cmd = wm.Update(logChunkMsg("gamma\n"))
	wm = next.(waitModel)
	if !wm.printing {
		t.Fatal("expected still printing")
	}
	if got := wm.pending; len(got) != 1 || got[0] != "gamma" {
		t.Fatalf("expected pending=[gamma], got %v", got)
	}
	if cmd != nil {
		t.Fatal("expected no new command while a print is in flight")
	}

	// When the in-flight print finishes, the buffered line flushes.
	next, cmd = wm.Update(printDoneMsg{})
	wm = next.(waitModel)
	if !wm.printing {
		t.Fatal("expected a new print for the buffered line")
	}
	if len(wm.pending) != 0 {
		t.Fatalf("expected pending drained, got %v", wm.pending)
	}
	if cmd == nil {
		t.Fatal("expected print command for buffered line")
	}

	// With nothing left buffered, the next done returns to idle.
	next, _ = wm.Update(printDoneMsg{})
	wm = next.(waitModel)
	if wm.printing {
		t.Fatal("expected printing=false when nothing buffered")
	}
}

func TestWaitModel_HoldsPartialLineUntilNewline(t *testing.T) {
	m := newWaitModel(internalbuild.Build{})

	// A chunk without a trailing newline isn't a complete line yet, so
	// nothing prints.
	next, cmd := m.Update(logChunkMsg("partial"))
	wm := next.(waitModel)
	if wm.printing || cmd != nil {
		t.Fatal("expected no print for an incomplete line")
	}
	if wm.leftover != "partial" {
		t.Fatalf("expected leftover=%q, got %q", "partial", wm.leftover)
	}

	// Completing the line triggers the print.
	next, cmd = wm.Update(logChunkMsg(" line\n"))
	wm = next.(waitModel)
	if !wm.printing || cmd == nil {
		t.Fatal("expected a print once the newline arrives")
	}
	if wm.leftover != "" {
		t.Fatalf("expected leftover consumed, got %q", wm.leftover)
	}
}

func TestWaitModel_FinalFlushEmitsTrailingPartialAndQuits(t *testing.T) {
	m := newWaitModel(internalbuild.Build{})

	// A trailing partial line (build's last line often has no newline).
	next, _ := m.Update(logChunkMsg("ExitCode: 0"))
	wm := next.(waitModel)

	next, cmd := wm.Update(watchDoneMsg{build: internalbuild.Build{Status: "success"}})
	wm = next.(waitModel)
	if !wm.finished {
		t.Fatal("expected finished=true")
	}
	if len(wm.pending) != 0 {
		t.Fatalf("expected pending flushed at exit, got %v", wm.pending)
	}
	if wm.leftover != "" {
		t.Fatalf("expected leftover flushed at exit, got %q", wm.leftover)
	}
	if cmd == nil {
		t.Fatal("expected a flush+quit command")
	}
}

func TestWaitModel_WaitsForInFlightPrintBeforeQuitting(t *testing.T) {
	m := newWaitModel(internalbuild.Build{})

	// Start a print.
	next, _ := m.Update(logChunkMsg("line\n"))
	wm := next.(waitModel)
	if !wm.printing {
		t.Fatal("expected printing")
	}

	// Build finishes while that print is still in flight: the model must
	// mark itself quitting but NOT issue the quit yet (that would race the
	// in-flight print and could drop or reorder the final lines).
	next, cmd := wm.Update(watchDoneMsg{})
	wm = next.(waitModel)
	if !wm.quitting {
		t.Fatal("expected quitting=true")
	}
	if cmd != nil {
		t.Fatal("expected nil command (waiting for the in-flight print)")
	}

	// Once the in-flight print signals done, the final flush + quit runs.
	_, cmd = wm.Update(printDoneMsg{})
	if cmd == nil {
		t.Fatal("expected flush+quit after the in-flight print completes")
	}
}
