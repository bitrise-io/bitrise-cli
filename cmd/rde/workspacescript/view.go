package workspacescript

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newViewCmd() *cobra.Command {
	var showSecrets bool
	c := &cobra.Command{
		Use:   "view",
		Short: "Show the workspace script, its state and its variables",
		Long: `Show the workspace script: whether it is enabled, who changed it last and
when, its variables and the script itself.

Secret variable values are hidden unless --show-secrets is given; protected
variables are write-only and never shown — only whether a value is set.`,
		Example: `  bitrise-cli rde workspace-script view
  bitrise-cli rde workspace-script view --show-secrets
  # Compare the live script with the one in git.
  bitrise-cli rde workspace-script view -o json | jq -r .script | diff - workspace-script.sh`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			ws, err := internalrde.NewService(client).GetWorkspaceScript(cmd.Context(), workspaceID, showSecrets)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, ws, detailRenderer(showSecrets))
		},
	}
	c.Flags().BoolVar(&showSecrets, "show-secrets", false, "show secret variable values (protected values are never returned)")
	return c
}

// detailRenderer renders the workspace script for humans. revealed says
// secret values were asked for, so an empty secret is shown as empty
// rather than hidden.
func detailRenderer(revealed bool) func(io.Writer, internalrde.WorkspaceScript) error {
	return func(w io.Writer, ws internalrde.WorkspaceScript) error {
		return renderDetail(w, ws, revealed)
	}
}

func renderDetail(w io.Writer, ws internalrde.WorkspaceScript, revealed bool) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	if !ws.Configured {
		ew.Ln("No workspace script saved yet.")
		ew.F("  %s\n", s.Dim.Render("Save one with 'bitrise-cli rde workspace-script update --script-file PATH'"))
		return ew.Err
	}
	lbl := func(label string) string { return s.Label.Render(fmt.Sprintf("%-16s", label)) }

	if ws.Enabled {
		ew.F("%s%s%s\n", lbl("Status:"), s.Success.Render("enabled"), s.Dim.Render(" — every session runs it at its next boot"))
	} else {
		ew.F("%s%s%s\n", lbl("Status:"), s.Warn.Render("disabled"), s.Dim.Render(" — draft: runs only on sessions created with --workspace-script-test"))
	}
	if ws.UpdatedBy != "" || ws.UpdatedAt != nil {
		changed := formatTime(ws.UpdatedAt)
		if ws.UpdatedBy != "" {
			if changed != "" {
				changed += " by "
			}
			changed += ws.UpdatedBy
		}
		ew.F("%s%s\n", lbl("Last changed:"), changed)
	}

	ew.Ln()
	ew.Ln(s.Dim.Render("Variables"))
	if ew.Err != nil {
		return ew.Err
	}
	if len(ws.Variables) == 0 {
		ew.F("  %s\n", s.Dim.Render("(none)"))
	} else {
		rows := make([][]string, 0, len(ws.Variables))
		for _, v := range ws.Variables {
			rows = append(rows, []string{v.Key, v.Visibility, variableValue(v, revealed)})
		}
		const colValue = 2
		styler := func(row, col int, content string) string {
			if col == colValue && content != ws.Variables[row].Value {
				return s.Dim.Render(content) // a placeholder, not a value
			}
			return content
		}
		if err := style.Table(w, []string{"KEY", "VISIBILITY", "VALUE"}, rows, s.Header, styler); err != nil {
			return err
		}
	}

	ew.Ln()
	ew.Ln(s.Dim.Render("Script"))
	if strings.TrimSpace(ws.Script) == "" {
		ew.F("  %s\n", s.Dim.Render("(empty)"))
		return ew.Err
	}
	// Verbatim, so it can be copied back out; JSON has it too ('.script').
	ew.F("%s", ws.Script)
	if !strings.HasSuffix(ws.Script, "\n") {
		ew.Ln()
	}
	return ew.Err
}

// variableValue is what the VALUE column shows: the value of a plain
// variable or a revealed secret, a placeholder otherwise.
func variableValue(v internalrde.WorkspaceScriptVariable, revealed bool) string {
	switch v.Visibility {
	case internalrde.VariableVisibilityProtected:
		if v.ValueStored {
			return "(set)"
		}
		return "(not set)"
	case internalrde.VariableVisibilitySecret:
		if !revealed {
			return "(hidden)"
		}
	}
	return v.Value
}

// formatTime is the canonical timestamp format for human output; nil
// renders as "".
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}
