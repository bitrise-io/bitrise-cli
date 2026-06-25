package cmd

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	"github.com/bitrise-io/bitrise-cli/internal/update"
)

// envNoUpdateNotifier disables the periodic "a new release is available" check
// when set to any non-empty value. Mirrors gh's GH_NO_UPDATE_NOTIFIER. The
// check is also skipped automatically in CI and on a non-interactive stderr.
const envNoUpdateNotifier = "BITRISE_CLI_NO_UPDATE_NOTIFIER"

// updateCheckTimeout caps the once-per-day GitHub round-trip. The check runs
// after the command's own output has been written, so this is the most a user
// ever waits on it (and only when the 24h cache is stale).
const updateCheckTimeout = 3 * time.Second

// updateNoticeArmed is set by persistentPreRun when the running command is
// eligible for an update notice. Execute reads it after the command finishes.
// It defaults to false, so a command that errors before PreRun simply gets no
// notice.
var updateNoticeArmed bool

// armUpdateCheck records, from the resolved settings and the running command,
// whether Execute should look for a newer release once the command is done.
func armUpdateCheck(cmd *cobra.Command, r config.Resolved) {
	updateNoticeArmed = shouldCheckForUpdate(
		r.Output,
		cmdutil.IsQuiet(cmd),
		cmdutil.IsTerminalWriter(cmd.ErrOrStderr()),
		cmd.Parent() != nil, // skip bare-root invocations (help, --version)
		os.Getenv,
		version,
		cmd.Name(),
	)
}

// shouldCheckForUpdate is the eligibility policy, factored out as a pure
// function so it can be tested without a cobra command. A notice is shown only
// for interactive, human-format use of a released binary that hasn't opted out.
func shouldCheckForUpdate(format output.Format, quiet, stderrIsTTY, isSubcommand bool, env func(string) string, currentVersion, cmdName string) bool {
	switch {
	case !isSubcommand: // bare `bitrise-cli`, help, --version: nothing to append a notice to
		return false
	case format != output.Human: // JSON is a machine contract — stay silent
		return false
	case quiet: // -q suppresses stderr diagnostics
		return false
	case !stderrIsTTY: // don't nag into pipes, files, or CI logs
		return false
	case isCI(env): // CI runs are scripts, not humans reading a prompt
		return false
	case env(envNoUpdateNotifier) != "": // explicit opt-out
		return false
	case !update.IsRelease(currentVersion): // dev builds have nothing to compare
		return false
	}
	switch cmdName {
	case "version", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		// `version` already shows it; completion output must not be perturbed.
		return false
	}
	return true
}

// isCI reports whether we're running inside a CI system, where an update nag is
// noise. CI sets `CI`; Bitrise builds also set `BITRISE_IO`.
func isCI(env func(string) string) bool {
	return env("CI") != "" || env("BITRISE_IO") != ""
}

// notifyUpdateAvailable runs the (usually cached) update check and prints a
// notice to w when a newer release exists. It is best-effort: any error is
// swallowed so a version check never affects the command's exit status, and a
// failed stderr write is ignored.
func notifyUpdateAvailable(w io.Writer) {
	if !updateNoticeArmed {
		return
	}
	checker, err := update.New(version)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()
	notice, _ := checker.Check(ctx)
	if notice == nil {
		return
	}
	renderUpdateNotice(w, notice)
}

// renderUpdateNotice writes the upgrade hint to stderr in the human scheme: a
// leading blank line to separate it from the command's output, the version
// delta, the release URL, and the one-line install command.
func renderUpdateNotice(w io.Writer, n *update.Notice) {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	ew.Ln()
	ew.F("%s %s %s %s\n",
		s.Warn.Render("A new release of bitrise-cli is available:"),
		s.Dim.Render(n.Current),
		"→",
		s.Bold.Render(n.Latest),
	)
	if n.URL != "" {
		ew.F("%s\n", s.URL.Render(n.URL))
	}
	ew.F("%s\n", s.Dim.Render("Upgrade: curl -fsSL https://app.bitrise.io/cli/install.sh | bash"))
	_ = ew.Err // best-effort; a failed stderr write must not change exit status
}
