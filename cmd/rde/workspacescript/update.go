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
Usage permission — Workspace API Tokens cannot edit the workspace script.

--set-var, --set-secret and --set-protected add a variable or replace an
existing one's value and visibility (plain, secret, protected). A value
given inline is visible in shell history and process arguments.`,
		Example: `  bitrise-cli rde workspace-script update --script-file workspace-script.sh
  cat workspace-script.sh | bitrise-cli rde workspace-script update --script-file -
  bitrise-cli rde workspace-script update --set-var REGION=eu --set-secret AGENT_TOKEN="$AGENT_TOKEN"
  bitrise-cli rde workspace-script update --remove-var OLD_FLAG
  # Try the draft on a test session, then turn it on for everyone.
  bitrise-cli rde workspace-script update --script-file workspace-script.sh --disable
  bitrise-cli rde session create try-it --workspace-script-test --template TEMPLATE_ID
  bitrise-cli rde workspace-script update --enable`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()
			var changes internalrde.WorkspaceScriptChanges
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
				for _, raw := range group.values {
					key, value, ok := strings.Cut(raw, "=")
					if !ok || key == "" {
						return fmt.Errorf("%s %q: expected KEY=VALUE", group.flag, raw)
					}
					changes.SetVariables = append(changes.SetVariables, internalrde.WorkspaceScriptVariableChange{
						Key: key, Value: value, Visibility: group.visibility,
					})
				}
			}
			changes.RemoveVariables = removeVars
			if flags.Changed("script-file") {
				script, err := readScript(cmd.InOrStdin(), scriptFile)
				if err != nil {
					return err
				}
				changes.Script = &script
			}
			if changes.IsEmpty() {
				return fmt.Errorf("nothing to update: pass at least one of --script-file, --enable, --disable, --set-var, --set-secret, --set-protected or --remove-var\nRun '%s --help' for usage", cmd.CommandPath())
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
	c.Flags().StringArrayVar(&setSecrets, "set-secret", nil, "set a secret variable as KEY=VALUE: encrypted at rest, hidden unless viewed with --show-secrets (repeatable)")
	c.Flags().StringArrayVar(&setProtected, "set-protected", nil, "set a protected variable as KEY=VALUE: encrypted and write-only, never shown again (repeatable)")
	c.Flags().StringArrayVar(&removeVars, "remove-var", nil, "remove the variable KEY (repeatable)")
	c.MarkFlagsMutuallyExclusive("enable", "disable")
	return c
}

// readScript reads the script from path; "-" reads stdin.
func readScript(stdin io.Reader, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("--script-file must not be empty (use '-' for stdin)")
	}
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path) //nolint:gosec // path comes from the user's --script-file flag
	}
	if err != nil {
		return "", fmt.Errorf("read script: %w", err)
	}
	return string(data), nil
}
