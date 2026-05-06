package cmd

import (
	"fmt"
	"io"
)

// errWriter wraps an io.Writer and captures the first write error.
type errWriter struct {
	w   io.Writer
	err error
}

func newErrWriter(w io.Writer) *errWriter {
	return &errWriter{w: w}
}

func (ew *errWriter) f(format string, a ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, a...)
}

func (ew *errWriter) ln(a ...any) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintln(ew.w, a...)
}
