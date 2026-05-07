package build

import (
	"bytes"
	"testing"
)

func TestContiguousSink_HoldsBackUntilContiguous(t *testing.T) {
	var buf bytes.Buffer
	sink := &contiguousSink{w: &buf}

	// Position 2 arrives before 1 — only position 0 is contiguous, hold 2.
	if err := sink.OnUpdate(map[int]string{0: "A\n", 2: "C\n"}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "A\n" {
		t.Errorf("after first update: got %q, want A\\n only (2 must wait for 1)", got)
	}

	// Position 1 fills the gap — flush 1 and 2 in order.
	if err := sink.OnUpdate(map[int]string{0: "A\n", 1: "B\n", 2: "C\n"}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "A\nB\nC\n" {
		t.Errorf("after gap fill: got %q, want A\\nB\\nC\\n", got)
	}

	// Subsequent calls don't re-emit — sink tracks how far it's emitted.
	if err := sink.OnUpdate(map[int]string{0: "A\n", 1: "B\n", 2: "C\n"}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "A\nB\nC\n" {
		t.Errorf("re-update should not re-emit: got %q", got)
	}
}

func TestContiguousSink_EmptyUpdateNoOp(t *testing.T) {
	// An empty poll (no chunks delivered yet) must not emit anything,
	// neither header-clearing escape codes nor placeholder bytes — the
	// header printed before streaming starts has to stay on screen.
	var buf bytes.Buffer
	sink := &contiguousSink{w: &buf}
	if err := sink.OnUpdate(map[int]string{}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty update should write nothing, got %q", buf.String())
	}
}

func TestContiguousSink_HoldsBackEntirelyWhenZeroMissing(t *testing.T) {
	// If the very first chunk to arrive is at position 5 (positions 0-4
	// still in flight), nothing should be emitted yet.
	var buf bytes.Buffer
	sink := &contiguousSink{w: &buf}
	if err := sink.OnUpdate(map[int]string{5: "F\n"}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("nothing should be emitted while position 0 is missing, got %q", buf.String())
	}
}
