package claude

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// stepLogger writes the command's grouped progress to stderr, following the
// output scheme (docs/output-scheme.md): a bold group title, indented step
// lines, and a green "✓" confirmation when a group completes. Warnings use the
// yellow Warn style. Everything is a diagnostic — it never touches stdout.
type stepLogger struct {
	w     io.Writer
	s     style.Styles
	quiet bool
}

func newStepLogger(cmd *cobra.Command) *stepLogger {
	w := cmd.ErrOrStderr()
	return &stepLogger{w: w, s: style.New(w), quiet: cmdutil.IsQuiet(cmd)}
}

// group starts a new section with a blank-line separator and a bold title.
func (l *stepLogger) group(title string) {
	if l.quiet {
		return
	}
	_, _ = fmt.Fprintf(l.w, "\n%s\n", l.s.Bold.Render(title))
}

// step reports an in-progress action within the current group (indented, dim).
func (l *stepLogger) step(format string, a ...any) {
	if l.quiet {
		return
	}
	_, _ = fmt.Fprintf(l.w, "  %s\n", l.s.Dim.Render(fmt.Sprintf(format, a...)))
}

// done confirms a completed group with the green "✓" success marker.
func (l *stepLogger) done(format string, a ...any) {
	if l.quiet {
		return
	}
	_, _ = fmt.Fprintf(l.w, "%s %s\n", l.s.Success.Render("✓"), fmt.Sprintf(format, a...))
}

// warn emits a yellow warning. Per the output scheme, warnings are not
// suppressed by --quiet.
func (l *stepLogger) warn(format string, a ...any) {
	_, _ = fmt.Fprintf(l.w, "  %s\n", l.s.Warn.Render(fmt.Sprintf(format, a...)))
}
