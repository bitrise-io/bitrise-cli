package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPurr_StaticToBuffer(t *testing.T) {
	// *bytes.Buffer is never a TTY → the static path runs even without
	// --once. Confirms that a non-TTY destination never animates and
	// always emits ANSI-free output.
	var buf bytes.Buffer
	if err := runPurr(t.Context(), &buf, false, time.Second, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-TTY output contains ANSI escape: %q", out)
	}
	if !strings.Contains(out, purrMessage) {
		t.Errorf("output missing message:\n%s", out)
	}
	// Cat ear lines + message + leading blank = exactly 12 lines.
	if got := strings.Count(out, "\n"); got != 12 {
		t.Errorf("line count = %d, want 12: %q", got, out)
	}
}

func TestPurr_OnceDoesntAnimate(t *testing.T) {
	var buf bytes.Buffer
	if err := runPurr(t.Context(), &buf, true, time.Hour, time.Microsecond); err != nil {
		t.Fatal(err)
	}
	// With once=true the function returns immediately after one paint;
	// it must not block on the ticker even with a microsecond interval.
	if !strings.Contains(buf.String(), "Purr Request") {
		t.Errorf("missing message: %q", buf.String())
	}
}

func TestPurrFrames_AllSameShape(t *testing.T) {
	// Animation looks ugly if frames have different heights or the same
	// content, so guard both invariants.
	if len(purrFrames) < 2 {
		t.Fatalf("expected at least 2 frames, got %d", len(purrFrames))
	}
	height := strings.Count(purrFrames[0], "\n")
	for i, f := range purrFrames {
		if h := strings.Count(f, "\n"); h != height {
			t.Errorf("frame[%d] has %d newlines, want %d", i, h, height)
		}
	}
	seen := make(map[string]bool)
	distinct := 0
	for _, f := range purrFrames {
		if !seen[f] {
			seen[f] = true
			distinct++
		}
	}
	if distinct < 2 {
		t.Errorf("frames are all identical — animation has no motion")
	}
}
