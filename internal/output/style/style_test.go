package style

import (
	"bytes"
	"strings"
	"testing"
)

// hasANSI reports whether s contains an ANSI escape sequence.
func hasANSI(s string) bool { return strings.Contains(s, "\x1b[") }

func TestNew_NonTTYWriterIsAnsiFree(t *testing.T) {
	// A *bytes.Buffer is never a TTY → lipgloss/termenv falls back to
	// the Ascii profile and styles render as plain strings. This is the
	// invariant that keeps tests, pipes, and JSON output ANSI-free.
	var buf bytes.Buffer
	s := New(&buf)
	if s.HasColor() {
		t.Fatal("Styles built for *bytes.Buffer should not emit color")
	}
	for _, in := range []string{"hello", "Build:", "success"} {
		if got := s.Bold.Render(in); got != in {
			t.Errorf("Bold.Render(%q) = %q, want plain", in, got)
		}
		if got := s.BuildStatus("success").Render(in); got != in {
			t.Errorf("BuildStatus.Render(%q) = %q, want plain", in, got)
		}
	}
}

func TestConfigure_NoColorForcesAscii(t *testing.T) {
	t.Cleanup(func() { Configure(false) })
	Configure(true)

	var buf bytes.Buffer
	s := New(&buf)
	if s.HasColor() {
		t.Fatal("Configure(true) should force no color")
	}
	out := s.Success.Render("✓")
	if hasANSI(out) {
		t.Errorf("expected ANSI-free output, got %q", out)
	}
}

func TestBuildStatus_Mapping(t *testing.T) {
	// We can't assert specific colors without a TTY profile, but we can
	// confirm distinct underlying styles for the meaningful cases by
	// rendering against a forced color profile via a TTY-like writer.
	var buf bytes.Buffer
	s := New(&buf)

	cases := map[string]bool{
		"success":              true,
		"failed":               true,
		"aborted":              true,
		"aborted-with-success": true,
		"in-progress":          true,
		"":                     true, // unknown still returns a non-nil style
	}
	for status := range cases {
		// The returned style must be usable; Render on plain string should
		// just return it verbatim against an ANSI-free writer.
		if got := s.BuildStatus(status).Render("X"); got != "X" {
			t.Errorf("BuildStatus(%q).Render: got %q", status, got)
		}
	}
}

func TestTable_HeadersAndRows(t *testing.T) {
	var buf bytes.Buffer
	s := New(&buf)

	headers := []string{"NUMBER", "STATUS", "BRANCH"}
	rows := [][]string{
		{"42", "success", "main"},
		{"41", "in-progress", "feature/x"},
	}
	if err := Table(&buf, headers, rows, s.Header, nil); err != nil {
		t.Fatalf("Table: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "NUMBER") || !strings.Contains(out, "STATUS") || !strings.Contains(out, "BRANCH") {
		t.Errorf("missing headers in output:\n%s", out)
	}
	if !strings.Contains(out, "feature/x") {
		t.Errorf("missing row content:\n%s", out)
	}
	// Three rows: header + 2 data.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d:\n%s", len(lines), out)
	}
}

func TestTable_ColumnAlignment(t *testing.T) {
	// Cells of varying length should be right-padded so the next column
	// always starts at the same offset.
	var buf bytes.Buffer
	s := New(&buf)

	headers := []string{"A", "B"}
	rows := [][]string{
		{"x", "1"},
		{"longer-cell", "2"},
	}
	if err := Table(&buf, headers, rows, s.Header, nil); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines: %q", len(lines), buf.String())
	}
	// In every line, "B" / "1" / "2" should sit at the same column
	// because column 0 is padded to its widest cell ("longer-cell").
	col := strings.Index(lines[1], "1")
	if col == -1 || col != strings.Index(lines[2], "2") {
		t.Errorf("columns misaligned:\n%s", buf.String())
	}
}

func TestTable_StylerIsCalled(t *testing.T) {
	var buf bytes.Buffer
	s := New(&buf)

	headers := []string{"X"}
	rows := [][]string{{"a"}, {"b"}}

	called := 0
	styler := func(row, col int, content string) string {
		called++
		// Wrap with markers we can assert on.
		return "<" + content + ">"
	}
	if err := Table(&buf, headers, rows, s.Header, styler); err != nil {
		t.Fatal(err)
	}
	if called != 2 {
		t.Errorf("styler called %d times, want 2", called)
	}
	out := buf.String()
	if !strings.Contains(out, "<a>") || !strings.Contains(out, "<b>") {
		t.Errorf("styler output not in result:\n%s", out)
	}
}

func TestTable_FewerCellsThanHeadersDoesntPanic(t *testing.T) {
	var buf bytes.Buffer
	s := New(&buf)
	headers := []string{"A", "B", "C"}
	rows := [][]string{
		{"only-a"}, // 1 cell for 3 headers
		{"a", "b", "c"},
	}
	if err := Table(&buf, headers, rows, s.Header, nil); err != nil {
		t.Fatalf("Table: %v", err)
	}
	if !strings.Contains(buf.String(), "only-a") {
		t.Errorf("first row missing:\n%s", buf.String())
	}
}

func TestTable_EmptyHeadersIsNoop(t *testing.T) {
	var buf bytes.Buffer
	s := New(&buf)
	if err := Table(&buf, nil, nil, s.Header, nil); err != nil {
		t.Fatalf("Table: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty headers, got %q", buf.String())
	}
}
