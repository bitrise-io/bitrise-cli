package build

import (
	"fmt"
	"io"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

func renderBuildText(w io.Writer, b internalbuild.Build) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-16s", label))
	}
	ew.F("%s#%d (%s)\n", lbl("Build:"), b.BuildNumber, s.Slug.Render(b.Slug))
	ew.F("%s%s\n", lbl("App:"), s.Slug.Render(b.AppSlug))
	ew.F("%s%s\n", lbl("Status:"), s.BuildStatus(b.Status).Render(b.Status))
	if b.StatusText != "" {
		ew.F("%s%s\n", lbl("Status Text:"), b.StatusText)
	}
	if b.AbortReason != "" {
		ew.F("%s%s\n", lbl("Abort Reason:"), b.AbortReason)
	}
	if b.IsOnHold {
		ew.F("%syes\n", lbl("On Hold:"))
	}
	if b.Rebuildable {
		ew.F("%syes\n", lbl("Rebuildable:"))
	}
	ew.F("%s%s\n", lbl("Workflow:"), b.Workflow)
	if b.PipelineWorkflowID != "" {
		ew.F("%s%s\n", lbl("Pipeline WF:"), b.PipelineWorkflowID)
	}
	ew.F("%s%s\n", lbl("Branch:"), b.Branch)
	if b.Tag != "" {
		ew.F("%s%s\n", lbl("Tag:"), b.Tag)
	}
	if b.PullRequestID != 0 {
		if b.PullRequestTargetBranch != "" {
			ew.F("%s#%d → %s\n", lbl("Pull Request:"), b.PullRequestID, b.PullRequestTargetBranch)
		} else {
			ew.F("%s#%d\n", lbl("Pull Request:"), b.PullRequestID)
		}
	}
	if b.PullRequestViewURL != "" {
		ew.F("%s%s\n", lbl("PR URL:"), s.URL.Render(b.PullRequestViewURL))
	}
	if b.CommitHash != "" {
		ew.F("%s%s\n", lbl("Commit:"), s.Slug.Render(b.CommitHash))
	}
	if b.CommitMessage != "" {
		ew.F("%s%s\n", lbl("Message:"), b.CommitMessage)
	}
	ew.F("%s%s\n", lbl("Triggered:"), b.TriggeredAt.Format("2006-01-02 15:04:05 MST"))
	if b.TriggeredBy != "" {
		ew.F("%s%s\n", lbl("Triggered By:"), b.TriggeredBy)
	}
	if b.FinishedAt != nil {
		ew.F("%s%s\n", lbl("Finished:"), b.FinishedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if b.StackIdentifier != "" {
		ew.F("%s%s\n", lbl("Stack:"), b.StackIdentifier)
	}
	if b.MachineTypeID != "" {
		ew.F("%s%s\n", lbl("Machine Type:"), b.MachineTypeID)
	}
	if b.CreditCost > 0 {
		ew.F("%s%d\n", lbl("Credit Cost:"), b.CreditCost)
	}
	if b.BuildURL != "" {
		ew.F("%s%s\n", lbl("URL:"), s.URL.Render(b.BuildURL))
	}
	return ew.Err
}
