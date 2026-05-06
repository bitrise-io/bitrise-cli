package cmd

import (
	"fmt"
	"io"

	"github.com/bitrise-io/bitrise-cli/internal/build"
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

// renderBuildText prints a single build as key/value lines.
// Shared by build trigger and build view.
func renderBuildText(w io.Writer, b build.Build) error {
	ew := newErrWriter(w)
	ew.f("Build:        #%d (%s)\n", b.BuildNumber, b.Slug)
	ew.f("App:          %s\n", b.AppSlug)
	ew.f("Status:       %s\n", b.Status)
	if b.StatusText != "" {
		ew.f("Status Text:  %s\n", b.StatusText)
	}
	ew.f("Workflow:     %s\n", b.Workflow)
	ew.f("Branch:       %s\n", b.Branch)
	if b.CommitHash != "" {
		ew.f("Commit:       %s\n", b.CommitHash)
	}
	if b.CommitMessage != "" {
		ew.f("Message:      %s\n", b.CommitMessage)
	}
	ew.f("Triggered:    %s\n", b.TriggeredAt.Format("2006-01-02 15:04:05 MST"))
	if b.FinishedAt != nil {
		ew.f("Finished:     %s\n", b.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if b.BuildURL != "" {
		ew.f("URL:          %s\n", b.BuildURL)
	}
	return ew.err
}
