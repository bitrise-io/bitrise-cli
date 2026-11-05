package warmpool

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view POOL_ID",
		Short: "Show a pool's status, limits, configuration and warm sessions",
		Long: `Show a pool: its live status (ready / warming inventory, sessions in
use against its max, lifetime claimed and cold counters, refusals in the
last 7 days, errors), the stored configuration with secret input values
hidden, and the warm sessions currently in the pool.

A growing cold count means sessions had to be created on demand because no
warm session was available — the warm sessions count is too low for the demand. A
growing refused count means the pool was full more often than its users can
bear — the max sessions is too low for the demand.`,
		Example: `  bitrise-cli rde pool view POOL_ID
  bitrise-cli rde pool view ios-devs --output json | jq .status`,
		Args: cmdutil.RequireArgs("POOL_ID"),
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
			warmPoolID, err := svc.ResolveWarmPoolID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			p, err := svc.GetWarmPool(cmd.Context(), workspaceID, warmPoolID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, p, renderDetail)
		},
	}
}

func renderDetail(w io.Writer, p internalrde.WarmPool) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string { return s.Label.Render(fmt.Sprintf("%-18s", label)) }

	ew.F("%s%s\n", lbl("Name:"), p.Name)
	ew.F("%s%s\n", lbl("ID:"), s.Slug.Render(p.ID))
	if p.TemplateName != "" {
		ew.F("%s%s\n", lbl("Template:"), p.TemplateName)
	} else if p.TemplateID != "" {
		ew.F("%s%s\n", lbl("Template:"), s.Dim.Render("(deleted)"))
	}
	if p.TemplateID != "" {
		ew.F("%s%s\n", lbl("Template ID:"), s.Slug.Render(p.TemplateID))
	}
	if p.CreatedByEmail != "" {
		ew.F("%s%s\n", lbl("Created by:"), p.CreatedByEmail)
	} else {
		ew.F("%s%s\n", lbl("Created by:"), s.Dim.Render("workspace API token"))
	}
	ew.F("%s%d%s\n", lbl("Warm sessions:"), p.PoolSize, s.Dim.Render(" (kept booted and ready to claim)"))
	if p.MaxSessions > 0 {
		ew.F("%s%d%s\n", lbl("Max sessions:"), p.MaxSessions, s.Dim.Render(" (sessions from the pool in use at once, warm ones included)"))
	} else {
		ew.F("%s%s\n", lbl("Max sessions:"), s.Dim.Render("no limit"))
	}
	if st := p.Status; st != nil {
		ew.F("%s%d ready, %d warming\n", lbl("Inventory:"), st.Ready, st.Warming)
		inUse := strconv.Itoa(st.InUse)
		if p.MaxSessions > 0 {
			inUse += " of " + strconv.Itoa(p.MaxSessions)
		}
		switch st.LimitState {
		case internalrde.LimitFull:
			inUse += s.Warn.Render(" — full: claims that find no warm session, restores and starts are refused until a session ends")
		case internalrde.LimitOverMax:
			inUse += s.Warn.Render(" — over max: nothing is terminated; claims are refused until enough sessions end")
		}
		ew.F("%s%s\n", lbl("In use:"), inUse)
		ew.F("%s%d claimed, %d cold%s\n", lbl("Lifetime:"), st.ClaimedTotal, st.ColdTotal,
			s.Dim.Render(" (cold = created on demand, no warm session was available)"))
		ew.F("%s%d%s\n", lbl("Refused (7 days):"), st.Refused7d, s.Dim.Render(" (claims and restores refused because the pool was full)"))
		if st.ConfigError != "" {
			ew.F("%s%s\n", lbl("Config error:"), s.Failure.Render(st.ConfigError))
			ew.F("%s%s\n", lbl(""), s.Dim.Render("the pool creates nothing until the configuration is fixed ('rde pool update')"))
		}
		if st.LastError != "" {
			ew.F("%s%s\n", lbl("Last error:"), s.Warn.Render(st.LastError))
		}
		if st.PausedUntil != nil {
			ew.F("%s%s%s\n", lbl("Paused until:"), formatTime(st.PausedUntil), s.Dim.Render(" — backing off after repeated failures"))
		}
	}
	if sf := p.SetupFailed; sf != nil {
		msg := sf.Message
		if msg == "" {
			msg = fmt.Sprintf("On create exited with code %d", sf.ExitCode)
		}
		ew.F("%s%s\n", lbl("Setup failed:"), s.Failure.Render(msg))
		ew.F("%s%s\n", lbl(""), s.Dim.Render(fmt.Sprintf("paused after three failed On create runs (%s); the last machine is kept as session %s — inspect its logs, fix the template, then 'rde pool retry'", formatTime(sf.FailedAt), sf.SessionID)))
	}
	if p.StackID != "" {
		ew.F("%s%s\n", lbl("Stack:"), p.StackID)
	}
	if p.MachineType != "" {
		ew.F("%s%s\n", lbl("Machine type:"), p.MachineType)
	}
	if p.Cluster != "" {
		ew.F("%s%s\n", lbl("Cluster:"), p.Cluster)
	}
	switch {
	case p.DeviceSpec != nil:
		ew.F("%s%s\n", lbl("Device:"), p.DeviceSpec.Summary())
	case p.NoDevice:
		ew.F("%s%s\n", lbl("Device:"), s.Dim.Render("none (the template's device is skipped)"))
	}
	if p.CreatedAt != nil {
		ew.F("%s%s\n", lbl("Created:"), formatTime(p.CreatedAt))
	}
	if p.UpdatedAt != nil {
		ew.F("%s%s\n", lbl("Updated:"), formatTime(p.UpdatedAt))
	}
	if len(p.SessionInputs) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Session inputs"))
		for _, in := range p.SessionInputs {
			val := in.Value
			switch {
			case in.SavedInputID != "":
				val = s.Dim.Render("saved input ") + s.Slug.Render(in.SavedInputID)
			case in.IsSecret:
				val = s.Dim.Render("(hidden)")
			}
			key := in.Key
			if in.Hook != "" {
				key += " (" + in.Hook + ")"
			}
			ew.F("%s %s\n", lbl("  "+key+":"), val)
		}
	}
	if len(p.EnabledFeatureFlagNames) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Feature flags"))
		for _, f := range p.EnabledFeatureFlagNames {
			ew.F("  %s\n", f)
		}
	}
	if p.Status != nil && len(p.Status.Sessions) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Warm sessions"))
		if ew.Err != nil {
			return ew.Err
		}
		headers := []string{"SESSION_ID", "STATE", "CREATED", "READY"}
		rows := make([][]string, 0, len(p.Status.Sessions))
		for _, sess := range p.Status.Sessions {
			rows = append(rows, []string{sess.SessionID, sess.State, formatTime(sess.CreatedAt), formatTime(sess.ReadyAt)})
		}
		const (
			colID    = 0
			colState = 1
		)
		styler := func(_, col int, content string) string {
			switch col {
			case colID:
				return s.Slug.Render(content)
			case colState:
				if content == "ready" {
					return s.Success.Render(content)
				}
				return s.Dim.Render(content)
			}
			return content
		}
		return style.Table(w, headers, rows, s.Header, styler)
	}
	return ew.Err
}

// formatTime is the canonical timestamp format for human output; a nil
// pointer renders as "" so it can go straight into a table cell.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}
