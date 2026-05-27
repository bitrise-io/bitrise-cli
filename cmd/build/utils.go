package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

const watchDivider = "─────────────────────────────────────────────────────────"

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

// runWatch is the shared implementation for `build watch` and `build trigger --watch`.
// It prints a header/footer to stderr and streams log content to logWriter.
// For output.JSON format it renders the final build record as JSON to
// cmd.OutOrStdout() instead of the text footer.
//
// In human format on an interactive terminal it switches to a TUI that
// pins a spinner + status bar to the bottom and streams logs above it.
func runWatch(cmd *cobra.Command, svc *internalbuild.Service, b internalbuild.Build, interval time.Duration, logWriter io.Writer, format output.Format) error {
	if format == output.Human && stdoutIsTTY(cmd) {
		return runWatchTUI(cmd, svc, b, interval)
	}

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
		detachEW.F("Use 'bitrise-cli build watch %s' to resume streaming.\n", b.Slug)
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
	if url := buildDetailURL(cmd, b); url != "" {
		footerEW.F("→ %s\n", url)
	}
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
	if b.BuildURL != "" {
		s += fmt.Sprintf("\n→ %s", b.BuildURL)
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

// buildDetailURL returns the web URL of the build's detail page. It prefers
// the URL the API supplied (set on triggered builds) and falls back to
// constructing one from the resolved web base URL when the record doesn't
// carry it — e.g. the View path used by `build watch`.
func buildDetailURL(cmd *cobra.Command, b internalbuild.Build) string {
	if b.BuildURL != "" {
		return b.BuildURL
	}
	if b.AppSlug == "" || b.Slug == "" {
		return ""
	}
	return fmt.Sprintf("%s/app/%s/build/%s", cmdutil.ResolveWebBaseURL(cmd), b.AppSlug, b.Slug)
}
