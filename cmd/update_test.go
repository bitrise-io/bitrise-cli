package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/update"
)

// eligibleArgs is the baseline "should notify" input: interactive, human
// output, a released build, on a normal subcommand, with no opt-out. Each test
// case flips exactly one field to assert that gate.
type eligibleArgs struct {
	format       output.Format
	quiet        bool
	stderrIsTTY  bool
	isSubcommand bool
	env          map[string]string
	version      string
	cmdName      string
}

func (a eligibleArgs) call() bool {
	return shouldCheckForUpdate(
		a.format, a.quiet, a.stderrIsTTY, a.isSubcommand,
		func(k string) string { return a.env[k] },
		a.version, a.cmdName,
	)
}

func baseEligible() eligibleArgs {
	return eligibleArgs{
		format:       output.Human,
		quiet:        false,
		stderrIsTTY:  true,
		isSubcommand: true,
		env:          map[string]string{},
		version:      "1.0.0",
		cmdName:      "list",
	}
}

func TestShouldCheckForUpdate(t *testing.T) {
	if !baseEligible().call() {
		t.Fatal("baseline inputs should be eligible")
	}

	cases := []struct {
		name   string
		mutate func(*eligibleArgs)
	}{
		{"json output", func(a *eligibleArgs) { a.format = output.JSON }},
		{"quiet", func(a *eligibleArgs) { a.quiet = true }},
		{"non-tty stderr", func(a *eligibleArgs) { a.stderrIsTTY = false }},
		{"bare root command", func(a *eligibleArgs) { a.isSubcommand = false }},
		{"CI env", func(a *eligibleArgs) { a.env["CI"] = "true" }},
		{"Bitrise CI env", func(a *eligibleArgs) { a.env["BITRISE_IO"] = "true" }},
		{"opt-out env", func(a *eligibleArgs) { a.env[envNoUpdateNotifier] = "1" }},
		{"dev build", func(a *eligibleArgs) { a.version = "dev" }},
		{"git-describe build", func(a *eligibleArgs) { a.version = "v1.0.0-3-gabc123" }},
		{"version command", func(a *eligibleArgs) { a.cmdName = "version" }},
		{"completion command", func(a *eligibleArgs) { a.cmdName = "completion" }},
		{"shell-complete request", func(a *eligibleArgs) { a.cmdName = cobra.ShellCompRequestCmd }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := baseEligible()
			c.mutate(&a)
			if a.call() {
				t.Errorf("%s should disable the update check", c.name)
			}
		})
	}
}

func TestRenderUpdateNotice(t *testing.T) {
	var buf bytes.Buffer // non-TTY → ANSI-free, easy to assert on
	renderUpdateNotice(&buf, &update.Notice{
		Current: "1.0.0",
		Latest:  "2.0.0",
		URL:     "https://github.com/bitrise-io/bitrise-cli/releases/tag/v2.0.0",
	})
	out := buf.String()

	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-TTY notice contains ANSI escape: %q", out)
	}
	if !strings.HasPrefix(out, "\n") {
		t.Errorf("notice should start with a blank line, got %q", out)
	}
	for _, want := range []string{
		"1.0.0", "→", "2.0.0",
		"https://github.com/bitrise-io/bitrise-cli/releases/tag/v2.0.0",
		"install.sh",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("notice missing %q:\n%s", want, out)
		}
	}
}

func TestRenderUpdateNotice_OmitsEmptyURL(t *testing.T) {
	var buf bytes.Buffer
	renderUpdateNotice(&buf, &update.Notice{Current: "1.0.0", Latest: "2.0.0"})
	// No release URL → only the version delta and the install hint (3 lines:
	// leading blank, version, install). The install line legitimately carries
	// the install.sh URL, so assert the release-link line is gone specifically.
	if strings.Contains(buf.String(), "releases") {
		t.Errorf("expected no release URL line when URL is empty:\n%s", buf.String())
	}
	if got := strings.Count(buf.String(), "\n"); got != 3 {
		t.Errorf("line count = %d, want 3 (blank + version + install):\n%s", got, buf.String())
	}
}
