package cmdutil

import (
	"bytes"
	"os"
	"testing"
)

func TestReaderTTYFd_NonTerminalInputs(t *testing.T) {
	// nil reader and non-File readers must report not-a-TTY so the
	// keypress goroutine never tries to put a non-terminal into raw mode.
	if _, ok := ReaderTTYFd(nil); ok {
		t.Error("ReaderTTYFd(nil) should return false")
	}
	if _, ok := ReaderTTYFd(&bytes.Buffer{}); ok {
		t.Error("ReaderTTYFd(*bytes.Buffer) should return false")
	}
	// A regular file (not a TTY) is an *os.File, but term.IsTerminal is
	// false → ReaderTTYFd should still return false.
	tmp, err := os.CreateTemp(t.TempDir(), "tty-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tmp.Close() })
	if _, ok := ReaderTTYFd(tmp); ok {
		t.Error("ReaderTTYFd(regular *os.File) should return false")
	}
}
