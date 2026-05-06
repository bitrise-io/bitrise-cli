package build

import (
	"io"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
)

func renderBuildText(w io.Writer, b internalbuild.Build) error {
	ew := cmdutil.NewErrWriter(w)
	ew.F("Build:        #%d (%s)\n", b.BuildNumber, b.Slug)
	ew.F("App:          %s\n", b.AppSlug)
	ew.F("Status:       %s\n", b.Status)
	if b.StatusText != "" {
		ew.F("Status Text:  %s\n", b.StatusText)
	}
	ew.F("Workflow:     %s\n", b.Workflow)
	ew.F("Branch:       %s\n", b.Branch)
	if b.CommitHash != "" {
		ew.F("Commit:       %s\n", b.CommitHash)
	}
	if b.CommitMessage != "" {
		ew.F("Message:      %s\n", b.CommitMessage)
	}
	ew.F("Triggered:    %s\n", b.TriggeredAt.Format("2006-01-02 15:04:05 MST"))
	if b.FinishedAt != nil {
		ew.F("Finished:     %s\n", b.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if b.BuildURL != "" {
		ew.F("URL:          %s\n", b.BuildURL)
	}
	return ew.Err
}
