package build

import (
	"io"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
)

func renderBuildText(w io.Writer, b internalbuild.Build) error {
	ew := cmdutil.NewErrWriter(w)
	ew.F("Build:          #%d (%s)\n", b.BuildNumber, b.Slug)
	ew.F("App:            %s\n", b.AppSlug)
	ew.F("Status:         %s\n", b.Status)
	if b.StatusText != "" {
		ew.F("Status Text:    %s\n", b.StatusText)
	}
	if b.AbortReason != "" {
		ew.F("Abort Reason:   %s\n", b.AbortReason)
	}
	if b.IsOnHold {
		ew.Ln("On Hold:        yes")
	}
	if b.Rebuildable {
		ew.Ln("Rebuildable:    yes")
	}
	ew.F("Workflow:       %s\n", b.Workflow)
	if b.PipelineWorkflowID != "" {
		ew.F("Pipeline WF:    %s\n", b.PipelineWorkflowID)
	}
	ew.F("Branch:         %s\n", b.Branch)
	if b.Tag != "" {
		ew.F("Tag:            %s\n", b.Tag)
	}
	if b.PullRequestID != 0 {
		if b.PullRequestTargetBranch != "" {
			ew.F("Pull Request:   #%d → %s\n", b.PullRequestID, b.PullRequestTargetBranch)
		} else {
			ew.F("Pull Request:   #%d\n", b.PullRequestID)
		}
	}
	if b.PullRequestViewURL != "" {
		ew.F("PR URL:         %s\n", b.PullRequestViewURL)
	}
	if b.CommitHash != "" {
		ew.F("Commit:         %s\n", b.CommitHash)
	}
	if b.CommitMessage != "" {
		ew.F("Message:        %s\n", b.CommitMessage)
	}
	ew.F("Triggered:      %s\n", b.TriggeredAt.Format("2006-01-02 15:04:05 MST"))
	if b.TriggeredBy != "" {
		ew.F("Triggered By:   %s\n", b.TriggeredBy)
	}
	if b.FinishedAt != nil {
		ew.F("Finished:       %s\n", b.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if b.StackIdentifier != "" {
		ew.F("Stack:          %s\n", b.StackIdentifier)
	}
	if b.MachineTypeID != "" {
		ew.F("Machine Type:   %s\n", b.MachineTypeID)
	}
	if b.CreditCost > 0 {
		ew.F("Credit Cost:    %d\n", b.CreditCost)
	}
	if b.BuildURL != "" {
		ew.F("URL:            %s\n", b.BuildURL)
	}
	return ew.Err
}
