package cmdutil

import (
	"io"
	"os"

	"golang.org/x/term"
)

// WriterIsTTY reports whether w is an *os.File pointing at a terminal. Any
// other writer (pipe, *bytes.Buffer, file handle) returns false.
func WriterIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors are small ints, no overflow risk
}

// ReaderTTYFd returns the file descriptor of r if it is an *os.File pointing
// at a terminal. The boolean is false otherwise (nil, *bytes.Buffer, pipe,
// closed file).
func ReaderTTYFd(r io.Reader) (int, bool) {
	if r == nil {
		return 0, false
	}
	f, ok := r.(*os.File)
	if !ok {
		return 0, false
	}
	fd := int(f.Fd()) //nolint:gosec // file descriptors are small ints, no overflow risk
	if !term.IsTerminal(fd) {
		return 0, false
	}
	return fd, true
}
