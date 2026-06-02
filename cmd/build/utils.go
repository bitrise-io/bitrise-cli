package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
// In human format on an interactive terminal it switches to a TUI that
// pins a spinner + status bar to the bottom and streams logs above it.
func runWatch(cmd *cobra.Command, svc *internalbuild.Service, b internalbuild.Build, interval time.Duration, logWriter io.Writer, format output.Format) error {
	if format == output.Human && cmdutil.WriterIsTTY(cmd.OutOrStdout()) {
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
		if err := output.Render(cmd.OutOrStdout(), format, finalBuild, renderBuildText); err != nil {
			return err
		}
	} else {
		footerEW := cmdutil.NewErrWriter(stderr)
		footerEW.F("\n%s\n", watchDivider)
		footerEW.F("Build #%d finished: %s%s\n", finalBuild.BuildNumber, finalBuild.Status, buildElapsed(finalBuild))
		if url := buildDetailURL(cmd, b); url != "" {
			footerEW.F("→ %s\n", url)
		}
		if footerEW.Err != nil {
			return footerEW.Err
		}
	}

	// The exit code reflects the build outcome in every mode, including
	// --output json: stdout already carries the build record above.
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

// runWatchTUI is the interactive variant of runWatch. It renders a permanent
// status bar (spinner + build info + clickable URL) at the bottom of the
// terminal while build logs scroll above it. When the build finishes the TUI
// exits cleanly and the caller renders the final summary.
func runWatchTUI(cmd *cobra.Command, svc *internalbuild.Service, b internalbuild.Build, interval time.Duration) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	m := newWaitModel(b)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithOutput(cmd.OutOrStdout()))

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		finalBuild, err := svc.Watch(ctx, b.AppSlug, b.Slug, &teaLogWriter{p: p}, interval)
		p.Send(watchDoneMsg{build: finalBuild, err: err})
	}()

	finalModel, err := p.Run()
	cancel()
	<-doneCh
	if err != nil {
		return fmt.Errorf("render wait UI: %w", err)
	}

	fm, ok := finalModel.(waitModel)
	if !ok {
		return fmt.Errorf("unexpected final model type %T", finalModel)
	}

	stderr := cmd.ErrOrStderr()
	// User interrupted (Ctrl-C, SIGTERM) before the watch returned a result.
	// fm.finished is only true after watchDoneMsg arrived and was handled;
	// fm.finalErr is context.Canceled when Watch itself observed the cancel.
	if !fm.finished || errors.Is(fm.finalErr, context.Canceled) {
		ew := cmdutil.NewErrWriter(stderr)
		ew.F("Detached — build is still running.\n")
		ew.F("Use 'bitrise-cli build watch %s' to resume streaming.\n", b.Slug)
		return ew.Err
	}
	if fm.finalErr != nil {
		return fm.finalErr
	}

	final := fm.finalBuild
	footerEW := cmdutil.NewErrWriter(stderr)
	footerEW.F("Build #%d finished: %s%s\n", final.BuildNumber, final.Status, buildElapsed(final))
	if url := buildDetailURL(cmd, b); url != "" {
		footerEW.F("→ %s\n", url)
	}
	if footerEW.Err != nil {
		return footerEW.Err
	}
	if final.Status != "success" && final.Status != "aborted-with-success" {
		cmdutil.SilenceRootErrors(cmd)
		return fmt.Errorf("build %s", final.Status)
	}
	return nil
}

// teaLogWriter adapts svc.Watch's io.Writer log sink to the bubbletea
// program. Each Write becomes a logChunkMsg the model splits into lines
// printed above the status bar via tea.Println.
type teaLogWriter struct {
	p *tea.Program
}

func (w *teaLogWriter) Write(p []byte) (int, error) {
	w.p.Send(logChunkMsg(string(p)))
	return len(p), nil
}

type logChunkMsg string

// printDoneMsg signals that the in-flight tea.Println sequence has finished,
// so the next batch of buffered log lines may be printed. See flushPending.
type printDoneMsg struct{}

type watchDoneMsg struct {
	build internalbuild.Build
	err   error
}

type tickMsg time.Time

type waitModel struct {
	build      internalbuild.Build
	spinner    spinner.Model
	leftover   string
	pending    []string
	printing   bool
	quitting   bool
	startedAt  time.Time
	finalBuild internalbuild.Build
	finalErr   error
	finished   bool
	width      int
	labelStyle lipgloss.Style
	dimStyle   lipgloss.Style
	urlStyle   lipgloss.Style
}

// bitrisePurple is the Bitrise brand purple, used for the spinner and the
// build URL in the status bar. lipgloss downsamples to 256/16-color
// automatically when the terminal can't render truecolor.
const bitrisePurple = lipgloss.Color("#7B61FF")

func newWaitModel(b internalbuild.Build) waitModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(bitrisePurple)
	started := b.TriggeredAt
	if started.IsZero() {
		started = time.Now()
	}
	return waitModel{
		build:      b,
		spinner:    sp,
		startedAt:  started,
		width:      80,
		labelStyle: lipgloss.NewStyle().Bold(true),
		dimStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		urlStyle:   lipgloss.NewStyle().Foreground(bitrisePurple).Underline(true),
	}
}

func (m waitModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, elapsedTick())
}

func (m waitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case logChunkMsg:
		m.leftover += string(msg)
		for {
			i := strings.IndexByte(m.leftover, '\n')
			if i < 0 {
				break
			}
			line := strings.TrimRight(m.leftover[:i], "\r")
			m.leftover = m.leftover[i+1:]
			m.pending = append(m.pending, line)
		}
		return m.flushPending()

	case printDoneMsg:
		m.printing = false
		if m.quitting {
			return m.flushFinal()
		}
		return m.flushPending()

	case watchDoneMsg:
		m.finalBuild = msg.build
		m.finalErr = msg.err
		m.finished = true
		m.quitting = true
		// Any trailing partial line (no terminating newline) is the last
		// thing to print.
		if m.leftover != "" {
			m.pending = append(m.pending, m.leftover)
			m.leftover = ""
		}
		// If a print is still in flight, wait for it; printDoneMsg will
		// run flushFinal once it completes. Otherwise flush + quit now.
		if m.printing {
			return m, nil
		}
		return m.flushFinal()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		if m.finished {
			return m, nil
		}
		return m, elapsedTick()
	}
	return m, nil
}

// flushPending prints all buffered complete lines as one ordered block,
// but only when no print is already in flight. Bubbletea runs commands
// returned from Update concurrently (tea.Batch makes no ordering promise,
// and even separate Update calls race), so emitting one tea.Println per
// line lets the scheduler interleave them — which is exactly the log
// scrambling we're fixing. Instead we serialize: at most one print
// command is outstanding, new lines accumulate in m.pending while it runs,
// and printDoneMsg releases the next block once the previous one lands.
func (m waitModel) flushPending() (tea.Model, tea.Cmd) {
	if m.printing || len(m.pending) == 0 {
		return m, nil
	}
	block := strings.Join(m.pending, "\n")
	m.pending = nil
	m.printing = true
	return m, tea.Sequence(
		tea.Println(block),
		func() tea.Msg { return printDoneMsg{} },
	)
}

// flushFinal prints any remaining buffered lines and quits. Called once the
// build is done and no print is in flight, so ordering is preserved.
func (m waitModel) flushFinal() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if len(m.pending) > 0 {
		cmds = append(cmds, tea.Println(strings.Join(m.pending, "\n")))
		m.pending = nil
	}
	cmds = append(cmds, tea.Quit)
	return m, tea.Sequence(cmds...)
}

func (m waitModel) View() string {
	if m.finished {
		return ""
	}

	info := strings.Builder{}
	info.WriteString(m.spinner.View())
	info.WriteString(" ")
	info.WriteString(m.labelStyle.Render("Building"))
	if m.build.BuildNumber > 0 {
		info.WriteString(m.dimStyle.Render(fmt.Sprintf(" #%d", m.build.BuildNumber)))
	}
	if m.build.Workflow != "" {
		info.WriteString("  ")
		info.WriteString(m.build.Workflow)
	}
	switch {
	case m.build.Branch != "":
		info.WriteString(m.dimStyle.Render(" on "))
		info.WriteString(m.build.Branch)
	case m.build.Tag != "":
		info.WriteString(m.dimStyle.Render(" tag "))
		info.WriteString(m.build.Tag)
	}
	info.WriteString(m.dimStyle.Render(fmt.Sprintf("  %s elapsed", m.elapsed())))

	if m.build.BuildURL == "" {
		return info.String()
	}
	// URL on its own line — most modern terminals make plain URLs Cmd-/Ctrl-
	// clickable, and a full-line URL avoids truncation on narrow terminals.
	url := m.dimStyle.Render("→ ") + m.urlStyle.Render(m.build.BuildURL)
	return info.String() + "\n" + url
}

func (m waitModel) elapsed() string {
	return time.Since(m.startedAt).Round(time.Second).String()
}

// elapsedTick redraws the status bar once per second so the elapsed-time
// counter advances even between log chunks.
func elapsedTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// ensure io.Writer interface.
var _ io.Writer = (*teaLogWriter)(nil)
