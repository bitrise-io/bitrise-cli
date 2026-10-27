package template

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view TEMPLATE_ID",
		Short: "Show details of a single template",
		Args:  cmdutil.RequireArgs("TEMPLATE_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			templateID, err := svc.ResolveTemplateID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			t, err := svc.GetTemplate(cmd.Context(), workspaceID, templateID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, t, renderDetail)
		},
	}
}

func renderDetail(w io.Writer, t internalrde.Template) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string { return s.Label.Render(fmt.Sprintf("%-18s", label)) }

	ew.F("%s%s\n", lbl("Name:"), t.Name)
	ew.F("%s%s\n", lbl("ID:"), s.Slug.Render(t.ID))
	if t.Description != "" {
		ew.F("%s%s\n", lbl("Description:"), t.Description)
	}
	if t.StackID != "" {
		ew.F("%s%s\n", lbl("Stack:"), t.StackID)
	}
	if t.MachineType != "" {
		ew.F("%s%s\n", lbl("Machine type:"), t.MachineType)
	}
	if t.WorkingDirectory != "" {
		ew.F("%s%s\n", lbl("Working dir:"), t.WorkingDirectory)
	}
	if t.LifecycleMode != "" {
		ew.F("%s%s\n", lbl("Lifecycle:"), lifecycleModeText(s, t))
	}
	if t.DeviceSpec != nil {
		ew.F("%s%s\n", lbl("Device:"), t.DeviceSpec.Summary())
	}
	if t.CreatedByEmail != "" {
		ew.F("%s%s\n", lbl("Owner:"), t.CreatedByEmail)
	}
	if t.UpdatedAt != nil {
		ew.F("%s%s\n", lbl("Updated:"), t.UpdatedAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	if h := t.Hooks; h != nil {
		ew.Ln()
		ew.Ln(s.Dim.Render("Lifecycle hooks"))
		for _, hook := range []struct{ name, script string }{
			{"On create", h.OnCreate}, {"On claim", h.OnClaim}, {"On restore", h.OnRestore},
		} {
			ew.F("  %-12s%s\n", hook.name, hookScriptSummary(s, hook.script))
		}
	}
	// Scopes matter on a hooks template only: a legacy template's values are
	// all shared, so tagging them would be noise.
	showScopes := t.LifecycleMode == "hooks"
	if len(t.SessionInputs) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Session inputs"))
		for _, i := range t.SessionInputs {
			var tags []string
			if i.Required {
				tags = append(tags, "required")
			}
			if showScopes {
				tags = append(tags, scopeTag(i.Hook))
			}
			ew.F("  %s%s\n", i.Key, tagList(tags))
			if i.Description != "" {
				ew.F("    %s\n", s.Dim.Render(i.Description))
			}
		}
	}
	if len(t.FeatureFlags) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Feature flags"))
		for _, f := range t.FeatureFlags {
			ew.F("  %s\n", f.Name)
		}
	}
	if len(t.TemplateVariables) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Variables"))
		for _, v := range t.TemplateVariables {
			var tags []string
			switch {
			case v.Protected:
				tags = append(tags, "protected")
			case v.IsSecret:
				tags = append(tags, "secret")
			}
			if showScopes {
				tags = append(tags, scopeTag(v.Hook))
			}
			ew.F("  %s%s\n", v.Key, tagList(tags))
		}
	}
	return ew.Err
}

// scopeTag renders a value's scope for a listing: "shared", "on create",
// "on claim" or "on restore".
func scopeTag(hook string) string {
	return strings.ToLower(internalrde.ScopeLabel(hook))
}

// tagList renders listing tags as " (a, b)", or "" when there are none.
func tagList(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return " (" + strings.Join(tags, ", ") + ")"
}

// lifecycleModeText renders how the template's scripts run. The mode is
// passed through verbatim (a value added after this code was written
// renders as-is).
func lifecycleModeText(s style.Styles, t internalrde.Template) string {
	switch t.LifecycleMode {
	case "hooks":
		return "hooks" + s.Dim.Render(" — On create / On claim / On restore run on the booted machine")
	case "legacy":
		return "legacy" + s.Dim.Render(" — warmup and startup scripts")
	}
	return t.LifecycleMode
}

// hookScriptSummary says whether a hook is set and how big it is; the text
// itself is in --output json.
func hookScriptSummary(s style.Styles, script string) string {
	if script == "" {
		return s.Dim.Render("(not set)")
	}
	lines := strings.Count(strings.TrimRight(script, "\n"), "\n") + 1
	if lines == 1 {
		return "1 line"
	}
	return fmt.Sprintf("%d lines", lines)
}
