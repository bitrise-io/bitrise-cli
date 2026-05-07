package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// purrFrames are the cat-mascot ASCII frames cycled by `bitrise-cli purr`.
// Each frame has the same shape; only the body+tail row differs to suggest a
// happily swinging tail. Keep all frames the same line count and column
// width so the in-place redraw doesn't jiggle.
var purrFrames = []string{
	purrFrame("--==>>"),
	purrFrame("~~==>>"),
	purrFrame("__==>>"),
	purrFrame("~~==>>"),
}

func purrFrame(tail string) string {
	return fmt.Sprintf(`
                  _______
                 /  o  o  \
                ;    \_/   ;
                 \        /
                  '------'
                __|        |__
               |   *  *  *  |%s
               |______________|
                  /\      /\
                 '  '    '  '
`, tail)
}

const purrMessage = "Purr Request is always here to help you!"

// ANSI control sequences used to drive the in-place animation. Kept as
// named constants because the bare escape codes are easy to misread.
const (
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
	ansiClearBelow = "\x1b[J"   // clear from cursor to end of screen
	ansiCursorPrev = "\x1b[%dF" // CPL: move cursor up N lines, to column 0
)

func newPurrCmd() *cobra.Command {
	var (
		once     bool
		duration time.Duration
		interval time.Duration
	)
	c := &cobra.Command{
		Use:   "purr",
		Short: "Visit Purr Request, the Bitrise CLI mascot",
		Long: `Visit Purr Request — the rocket-powered cat that's always here to help you.

The mascot animates with a swinging tail. The animation runs for --duration
(default 8s) or until Ctrl-C; --once disables animation and prints a single
frame. When stdout is not a terminal (piped output, log file) the command
always prints once and exits regardless of --once.`,
		Example: `  bitrise-cli purr
  bitrise-cli purr --duration 30s
  bitrise-cli purr --once`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPurr(cmd.Context(), cmd.OutOrStdout(), once, duration, interval)
		},
	}
	c.Flags().BoolVar(&once, "once", false, "print a single frame instead of animating")
	c.Flags().DurationVar(&duration, "duration", 8*time.Second, "how long to animate before exiting")
	c.Flags().DurationVar(&interval, "interval", 250*time.Millisecond, "delay between animation frames")
	return c
}

func runPurr(ctx context.Context, out io.Writer, once bool, duration, interval time.Duration) error {
	// Static path: piped output, --once, or stdout isn't a TTY.
	if once || !writerIsTTY(out) {
		if _, err := fmt.Fprint(out, purrFrames[0]); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, purrMessage)
		return err
	}

	// Hide cursor during animation, restore even on early exit.
	if _, err := fmt.Fprint(out, ansiHideCursor); err != nil {
		return err
	}
	defer func() { _, _ = fmt.Fprint(out, ansiShowCursor) }()

	// Stop cleanly on Ctrl-C.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	// Initial paint.
	if _, err := fmt.Fprint(out, purrFrames[0]); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, purrMessage); err != nil {
		return err
	}

	// One \n in the frame string + one from Fprintln(message) gives the
	// total number of cursor-down moves we need to undo each tick.
	height := strings.Count(purrFrames[0], "\n") + 1

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	deadline := time.NewTimer(duration)
	defer deadline.Stop()

	frame := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-deadline.C:
			return nil
		case <-ticker.C:
			frame = (frame + 1) % len(purrFrames)
			// Move cursor back to the top of the frame, clear everything below,
			// and redraw the frame + message.
			if _, err := fmt.Fprintf(out, ansiCursorPrev, height); err != nil {
				return err
			}
			if _, err := fmt.Fprint(out, ansiClearBelow); err != nil {
				return err
			}
			if _, err := fmt.Fprint(out, purrFrames[frame]); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out, purrMessage); err != nil {
				return err
			}
		}
	}
}

// writerIsTTY reports whether w is an *os.File pointing at a terminal. Any
// other writer (pipe, *bytes.Buffer, file handle) returns false so the
// caller takes the static, ANSI-free path.
func writerIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors are small ints, no overflow risk
}
