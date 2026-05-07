package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
)

// stdoutIsTTY reports whether cmd's stdout is an interactive terminal that
// can host the bubbletea TUI. Pipes, files, and *bytes.Buffer fail here.
func stdoutIsTTY(cmd *cobra.Command) bool {
	f, ok := cmd.OutOrStdout().(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// runWatchTUI is the interactive variant of runWatch. It renders a permanent
// status bar (spinner + build info + clickable URL) at the bottom of the
// terminal while build logs scroll above it. When the build finishes the TUI
// exits cleanly and the caller renders the final summary.
func runWatchTUI(cmd *cobra.Command, svc *internalbuild.Service, b internalbuild.Build, interval time.Duration, verbose bool) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	m := newWaitModel(b, verbose)
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
		ew.F("Use 'bitrise-cli build view %s' to check status.\n", b.Slug)
		return ew.Err
	}
	if fm.finalErr != nil {
		return fm.finalErr
	}

	final := fm.finalBuild
	footerEW := cmdutil.NewErrWriter(stderr)
	footerEW.F("Build #%d finished: %s%s\n", final.BuildNumber, final.Status, buildElapsed(final))
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

type watchDoneMsg struct {
	build internalbuild.Build
	err   error
}

type tickMsg time.Time

type waitModel struct {
	build      internalbuild.Build
	spinner    spinner.Model
	leftover   string
	startedAt  time.Time
	finalBuild internalbuild.Build
	finalErr   error
	finished   bool
	verbose    bool
	width      int
	labelStyle lipgloss.Style
	dimStyle   lipgloss.Style
	urlStyle   lipgloss.Style
}

// bitrisePurple is the Bitrise brand purple, used for the spinner and the
// build URL in the status bar. lipgloss downsamples to 256/16-color
// automatically when the terminal can't render truecolor.
const bitrisePurple = lipgloss.Color("#7B61FF")

func newWaitModel(b internalbuild.Build, verbose bool) waitModel {
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
		verbose:    verbose,
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
		if m.verbose {
			m.leftover += string(msg)
			var cmds []tea.Cmd
			for {
				i := strings.IndexByte(m.leftover, '\n')
				if i < 0 {
					break
				}
				line := strings.TrimRight(m.leftover[:i], "\r")
				m.leftover = m.leftover[i+1:]
				cmds = append(cmds, tea.Println(line))
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case watchDoneMsg:
		m.finalBuild = msg.build
		m.finalErr = msg.err
		m.finished = true
		var cmds []tea.Cmd
		if m.verbose && m.leftover != "" {
			cmds = append(cmds, tea.Println(m.leftover))
			m.leftover = ""
		}
		cmds = append(cmds, tea.Quit)
		return m, tea.Sequence(cmds...)

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
