package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

const watchDivider = "─────────────────────────────────────────────────────────"

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

// runWatch is the shared implementation for `build watch` and `build trigger --wait`.
// It prints a header/footer to stderr and streams log content to logWriter.
// For output.JSON format it renders the final build record as JSON to
// cmd.OutOrStdout() instead of the text footer.
func runWatch(cmd *cobra.Command, svc *internalbuild.Service, b internalbuild.Build, interval time.Duration, logWriter io.Writer, format output.Format) error {
	stderr := cmd.ErrOrStderr()

	headerEW := cmdutil.NewErrWriter(stderr)
	headerEW.F("%s\n", buildWatchHeader(b))
	headerEW.F("%s\n", watchDivider)
	if headerEW.Err != nil {
		return headerEW.Err
	}

	finalBuild, err := svc.Watch(cmd.Context(), b.AppSlug, b.Slug, logWriter, interval)
	if errors.Is(err, context.Canceled) {
		detachEW := cmdutil.NewErrWriter(stderr)
		detachEW.F("\nDetached — build is still running.\n")
		detachEW.F("Use 'bitrise-cli build view %s' to check status.\n", b.Slug)
		return detachEW.Err
	}
	if err != nil {
		return err
	}

	if format == output.JSON {
		return output.Render(cmd.OutOrStdout(), format, finalBuild, renderBuildText)
	}

	footerEW := cmdutil.NewErrWriter(stderr)
	footerEW.F("\n%s\n", watchDivider)
	footerEW.F("Build #%d finished: %s%s\n", finalBuild.BuildNumber, finalBuild.Status, buildElapsed(finalBuild))
	if footerEW.Err != nil {
		return footerEW.Err
	}

	if finalBuild.Status != "success" && finalBuild.Status != "aborted-with-success" {
		cmdutil.SilenceRootErrors(cmd)
		return fmt.Errorf("build %s", finalBuild.Status)
	}
	return nil
}

func buildWatchHeader(b internalbuild.Build) string {
	s := fmt.Sprintf("Watching build #%d", b.BuildNumber)
	if b.Workflow != "" {
		s += fmt.Sprintf(" — workflow '%s'", b.Workflow)
	}
	if b.Branch != "" {
		s += fmt.Sprintf(" on branch '%s'", b.Branch)
	} else if b.Tag != "" {
		s += fmt.Sprintf(" on tag '%s'", b.Tag)
	}
	return s
}

func buildElapsed(b internalbuild.Build) string {
	if b.FinishedAt == nil || b.TriggeredAt.IsZero() {
		return ""
	}
	d := b.FinishedAt.Sub(b.TriggeredAt).Round(time.Second)
	return fmt.Sprintf(" (%s)", d)
}
