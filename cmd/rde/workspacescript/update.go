package workspacescript

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newUpdateCmd() *cobra.Command {
	var (
		scriptFile   string
		enable       bool
		disable      bool
		setVars      []string
		setSecrets   []string
		setProtected []string
		removeVars   []string
	)
	c := &cobra.Command{
		Use:   "update",
		Short: "Change the workspace script, its state or its variables",
		Long: `Change the workspace script: the script every session in the workspace
runs first at each boot (see 'bitrise-cli rde workspace-script --help').

Only what the flags name changes; everything else is kept. The CLI reads the
current script (secret values included), applies the changes and saves the
whole script back. Protected variables you don't touch keep their stored
value.

Saving changes what every member's sessions run at their next boot (while
the script is enabled). Test a change first: save it disabled, create a test
session with 'bitrise-cli rde session create NAME --workspace-script-test',
check the "[workspace]" lines in its System log, then --enable.

This suits keeping the script in git and deploying it from CI with
--script-file and a personal access token of someone with the workspace's
Usage permission. Workspace API Tokens cannot edit the workspace script by
default; if your CI needs one for this, contact Bitrise.

--set-var, --set-secret and --set-protected add a variable or replace an
existing one's value and visibility (plain, secret, protected).

Pass secret and protected values on stdin: --set-secret KEY=- (or
--set-protected KEY=-) reads the value from stdin, with one trailing newline
trimmed, and keeps it out of shell history and process arguments. A value
given inline is visible in both. Only one thing can read stdin per call: a
single KEY=- value or --script-file -.`,
		Example: `  bitrise-cli rde workspace-script update --script-file workspace-script.sh
  cat workspace-script.sh | bitrise-cli rde workspace-script update --script-file -
  bitrise-cli rde workspace-script update --set-var REGION=eu
  # Secrets from stdin, not the command line (e.g. from a CI secret env var).
  printf '%s' "$AGENT_TOKEN" | bitrise-cli rde workspace-script update --set-secret AGENT_TOKEN=-
  bitrise-cli rde workspace-script update --remove-var OLD_FLAG
  # Try the draft on a test session, then turn it on for everyone.
  bitrise-cli rde workspace-script update --script-file workspace-script.sh --disable
  bitrise-cli rde session create try-it --workspace-script-test --template TEMPLATE_ID
  bitrise-cli rde workspace-script update --enable`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()
			var (
				changes internalrde.WorkspaceScriptChanges
				// stdinUsers names what reads stdin; at most one may.
				stdinUsers []string
				// stdinVar is the index in changes.SetVariables of the
				// variable whose value comes from stdin, or -1.
				stdinVar = -1
			)
			if flags.Changed("enable") {
				v := enable
				changes.Enabled = &v
			}
			if flags.Changed("disable") {
				v := !disable
				changes.Enabled = &v
			}
			for _, group := range []struct {
				flag       string
				values     []string
				visibility string
			}{
				{"--set-var", setVars, internalrde.VariableVisibilityPlain},
				{"--set-secret", setSecrets, internalrde.VariableVisibilitySecret},
				{"--set-protected", setProtected, internalrde.VariableVisibilityProtected},
			} {
				for i, raw := range group.values {
					key, value, ok := strings.Cut(raw, "=")
					if !ok || key == "" {
						// Never echo the argument: it may be a secret.
						return fmt.Errorf("%s (occurrence %d): expected KEY=VALUE\nRun '%s --help' for usage", group.flag, i+1, cmd.CommandPath())
					}
					if value == stdinValue && group.visibility != internalrde.VariableVisibilityPlain {
						stdinUsers = append(stdinUsers, fmt.Sprintf("%s %s=-", group.flag, key))
						stdinVar = len(changes.SetVariables)
					}
					changes.SetVariables = append(changes.SetVariables, internalrde.WorkspaceScriptVariableChange{
						Key: key, Value: value, Visibility: group.visibility,
					})
				}
			}
			changes.RemoveVariables = removeVars
			if flags.Changed("script-file") {
				if scriptFile == "" {
					return fmt.Errorf("--script-file must not be empty (use '-' for stdin)")
				}
				if scriptFile == stdinValue {
					stdinUsers = append(stdinUsers, "--script-file -")
				}
				changes.Script = &scriptFile // replaced by the content below
			}
			if len(stdinUsers) > 1 {
				return fmt.Errorf("only one value can be read from stdin per call, got %s\nRun '%s --help' for usage", strings.Join(stdinUsers, " and "), cmd.CommandPath())
			}
			if changes.IsEmpty() {
				return fmt.Errorf("nothing to update: pass at least one of --script-file, --enable, --disable, --set-var, --set-secret, --set-protected or --remove-var\nRun '%s --help' for usage", cmd.CommandPath())
			}
			// Everything checkable without the stored script fails here,
			// before stdin is read and before any request.
			if err := changes.Validate(); err != nil {
				return err
			}
			if changes.Script != nil {
				script, err := readScript(cmd.InOrStdin(), scriptFile)
				if err != nil {
					return err
				}
				changes.Script = &script
			}
			if stdinVar >= 0 {
				v, err := readStdinValue(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("%s: %w", stdinUsers[0], err)
				}
				if v == "" {
					return fmt.Errorf("%s: no value read from stdin", stdinUsers[0])
				}
				changes.SetVariables[stdinVar].Value = v
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			ws, err := internalrde.NewService(client).UpdateWorkspaceScript(cmd.Context(), workspaceID, changes)
			if err != nil {
				return err
			}
			if err := output.Render(cmd.OutOrStdout(), format, ws, detailRenderer(false)); err != nil {
				return err
			}
			if !cmdutil.IsQuiet(cmd) {
				msg := "Saved workspace script; sessions run it from their next boot"
				if !ws.Enabled {
					msg = "Saved workspace script (disabled: runs only on sessions created with --workspace-script-test)"
				}
				_, err := fmt.Fprintln(cmd.ErrOrStderr(), msg)
				return err
			}
			return nil
		},
	}
	c.Flags().StringVar(&scriptFile, "script-file", "", "path to the bash script to save ('-' reads stdin)")
	c.Flags().BoolVar(&enable, "enable", false, "enable the script: every session runs it at its next boot")
	c.Flags().BoolVar(&disable, "disable", false, "disable the script (draft): only sessions created with --workspace-script-test run it")
	c.Flags().StringArrayVar(&setVars, "set-var", nil, "set a plain variable as KEY=VALUE (repeatable)")
	c.Flags().StringArrayVar(&setSecrets, "set-secret", nil, "set a secret variable as KEY=VALUE: encrypted at rest, hidden unless viewed with --show-secrets; KEY=- reads the value from stdin (recommended) (repeatable)")
	c.Flags().StringArrayVar(&setProtected, "set-protected", nil, "set a protected variable as KEY=VALUE: encrypted and write-only, never shown again; KEY=- reads the value from stdin (recommended) (repeatable)")
	c.Flags().StringArrayVar(&removeVars, "remove-var", nil, "remove the variable KEY (repeatable)")
	c.MarkFlagsMutuallyExclusive("enable", "disable")
	return c
}

// stdinValue is the flag value that means "read it from stdin".
const stdinValue = "-"

// readScript reads the script from path; "-" reads stdin.
func readScript(stdin io.Reader, path string) (string, error) {
	var (
		data []byte
		err  error
	)
	if path == stdinValue {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path) //nolint:gosec // path comes from the user's --script-file flag
	}
	if err != nil {
		return "", fmt.Errorf("read script: %w", err)
	}
	return string(data), nil
}

// readStdinValue reads a variable value from stdin with one trailing newline
// trimmed, as 'bitrise-cli config set KEY -' does.
func readStdinValue(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read value from stdin: %w", err)
	}
	s := strings.TrimSuffix(string(data), "\n")
	return strings.TrimSuffix(s, "\r"), nil
}
