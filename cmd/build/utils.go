package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
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
// Log chunks come back from the API in arbitrary order across polls
// (parallel server-side producers); we mirror the website's approach by
// keying chunks on Position and rendering the cumulative sorted log:
//   - on a TTY: clear the screen and reprint the full log on each update
//   - off a TTY (pipes/files): append only chunks that extend the
//     contiguous run from position 0 — preserves order without rewinding
func runWatch(cmd *cobra.Command, svc *internalbuild.Service, b internalbuild.Build, interval time.Duration, logWriter io.Writer, format output.Format) error {
	stderr := cmd.ErrOrStderr()

	headerEW := cmdutil.NewErrWriter(stderr)
	headerEW.F("%s\n", buildWatchHeader(b))
	headerEW.F("%s\n", watchDivider)
	if headerEW.Err != nil {
		return headerEW.Err
	}

	sink := newLogSinkFor(logWriter)
	finalBuild, err := svc.Watch(cmd.Context(), b.AppSlug, b.Slug, logWriter, sink, interval)
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

// newLogSinkFor returns a LogSink appropriate for the given writer:
//   - clearTTYSink when the writer is a TTY (clear + reprint on each update)
//   - contiguousSink otherwise (append once a position is contiguous from 0)
func newLogSinkFor(w io.Writer) internalbuild.LogSink {
	if cmdutil.WriterIsTTY(w) {
		return &clearTTYSink{w: w}
	}
	return &contiguousSink{w: w}
}

// clearTTYSink wipes the terminal and reprints the full log sorted by
// position on every update. This matches the website's polling behavior:
// each poll's batch may carry chunks at higher positions than missing
// earlier ones, and re-rendering from scratch keeps the visible output
// in canonical order even as gaps fill in.
type clearTTYSink struct {
	w io.Writer
}

// ANSI clear sequence:
//
//	ESC[H   home the cursor (top-left)
//	ESC[2J  clear the visible viewport
//	ESC[3J  clear the scrollback buffer (xterm extension)
//
// Without ESC[3J most modern terminals (macOS Terminal, iTerm2, GNOME Terminal)
// push cleared content into the scrollback buffer instead of discarding it,
// so the user sees every redraw stacked on top of the previous ones when they
// scroll up. The order matters: ESC[3J after ESC[2J on the empty viewport
// drops the current screen out of scrollback as well.
const ansiClearAndHome = "\033[H\033[2J\033[3J"

func (s *clearTTYSink) OnUpdate(chunks map[int]string) error {
	if len(chunks) == 0 {
		return nil
	}
	maxPos := -1
	for k := range chunks {
		if k > maxPos {
			maxPos = k
		}
	}
	var b strings.Builder
	b.Grow(len(ansiClearAndHome) + (maxPos+1)*32)
	b.WriteString(ansiClearAndHome)
	for i := 0; i <= maxPos; i++ {
		// Missing positions render as empty strings — placeholders the
		// next update will fill in once the chunk arrives.
		b.WriteString(chunks[i])
	}
	_, err := io.WriteString(s.w, b.String())
	return err
}

// contiguousSink appends chunks to w only once they extend the contiguous
// run from position 0. A non-TTY consumer (file, pipe, log aggregator)
// can't undo writes, so we hold back chunks at higher positions until
// every preceding position has arrived. This trades latency for correct
// in-order output.
type contiguousSink struct {
	w        io.Writer
	nextEmit int
}

func (s *contiguousSink) OnUpdate(chunks map[int]string) error {
	for {
		c, ok := chunks[s.nextEmit]
		if !ok {
			return nil
		}
		if _, err := io.WriteString(s.w, c); err != nil {
			return fmt.Errorf("write log chunk: %w", err)
		}
		s.nextEmit++
	}
}
