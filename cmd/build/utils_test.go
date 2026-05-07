package build

import (
	"bytes"
	"strings"
	"testing"
)

func TestClearTTYSink_PrintsClearAndFullSortedLog(t *testing.T) {
	var buf bytes.Buffer
	sink := &clearTTYSink{w: &buf}

	// First update: positions 0, 2 (gap at 1).
	if err := sink.OnUpdate(map[int]string{0: "A\n", 2: "C\n"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.HasPrefix(got, ansiClearAndHome) {
		t.Errorf("first update missing clear sequence: %q", got)
	}
	// Verify both viewport (ESC[2J) and scrollback (ESC[3J) clears are present.
	if !strings.Contains(got, "\033[2J") || !strings.Contains(got, "\033[3J") {
		t.Errorf("clear sequence missing viewport or scrollback wipe: %q", got)
	}
	// Gap at position 1 renders as empty — output is "A\n" + "" + "C\n".
	if !strings.HasSuffix(got, "A\nC\n") {
		t.Errorf("first update body = %q, want suffix A\\nC\\n", got)
	}

	// Second update: position 1 fills the gap.
	buf.Reset()
	if err := sink.OnUpdate(map[int]string{0: "A\n", 1: "B\n", 2: "C\n"}); err != nil {
		t.Fatal(err)
	}
	got = buf.String()
	if !strings.HasPrefix(got, ansiClearAndHome) {
		t.Errorf("second update missing clear sequence")
	}
	if !strings.HasSuffix(got, "A\nB\nC\n") {
		t.Errorf("second update body = %q, want suffix A\\nB\\nC\\n", got)
	}
}

func TestClearTTYSink_NoOpOnEmpty(t *testing.T) {
	var buf bytes.Buffer
	sink := &clearTTYSink{w: &buf}
	if err := sink.OnUpdate(map[int]string{}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty update should write nothing, got %q", buf.String())
	}
}

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

	// Position 1 fills the gap — flush 1 and 2.
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
