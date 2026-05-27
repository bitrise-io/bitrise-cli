package session

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newViewCmd() *cobra.Command {
	var (
		watch    bool
		interval time.Duration
	)
	c := &cobra.Command{
		Use:   "view SESSION_ID",
		Short: "Show details of a single session",
		Long: `Show details of a single session.

Pass --watch to poll the session and re-render on every change until you
hit Ctrl-C — useful while waiting for a session to come up. --watch is
incompatible with --output json (the contract is a single object, not a
stream); use 'session create --wait' or a polling jq loop instead.`,
		Args: cmdutil.RequireArgs("SESSION_ID"),
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

			if !watch {
				sess, err := svc.GetSession(cmd.Context(), workspaceID, args[0])
				if err != nil {
					return err
				}
				return output.Render(cmd.OutOrStdout(), format, sess, renderSessionDetail)
			}

			if format == output.JSON {
				return fmt.Errorf("--watch cannot be combined with --output json (JSON is a single-object contract)")
			}
			return watchSession(cmd, svc, workspaceID, args[0], interval)
		},
	}
	c.Flags().BoolVar(&watch, "watch", false, "poll the session and re-render on every change until Ctrl-C")
	c.Flags().DurationVar(&interval, "interval", 3*time.Second, "polling interval when --watch is set (Go duration syntax: 1s, 500ms, …)")
	return c
}

// watchSession loops on GetSession + render until ctx is cancelled.
// Output is ANSI-cleared each iteration so the terminal shows just the
// latest state. Failures during the loop are surfaced and abort the
// watch — that's better than silently retrying past a permissions/404
// error the user should know about.
func watchSession(cmd *cobra.Command, svc *internalrde.Service, workspaceID, sessionID string, interval time.Duration) error {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	out := cmd.OutOrStdout()
	first := true
	for {
		sess, err := svc.GetSession(cmd.Context(), workspaceID, sessionID)
		if err != nil {
			if errors.Is(err, cmd.Context().Err()) {
				return nil
			}
			return err
		}
		if !first {
			// ANSI: cursor to top-left + clear screen below. Skipping on
			// the first iteration keeps any prior shell prompt visible
			// above the watch output.
			_, _ = fmt.Fprint(out, "\x1b[H\x1b[J")
		}
		first = false
		if err := renderSessionDetail(out, sess); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "\nwatching every %s — Ctrl-C to exit\n", interval)
		select {
		case <-cmd.Context().Done():
			return nil
		case <-time.After(interval):
		}
	}
}

func renderSessionDetail(w io.Writer, sess internalrde.Session) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string { return s.Label.Render(fmt.Sprintf("%-22s", label)) }

	ew.F("%s%s\n", lbl("Name:"), sess.Name)
	ew.F("%s%s\n", lbl("ID:"), s.Slug.Render(sess.ID))
	if sess.Description != "" {
		ew.F("%s%s\n", lbl("Description:"), sess.Description)
	}
	if sess.Status != "" {
		ew.F("%s%s\n", lbl("Status:"), statusStyle(s, sess.Status).Render(sess.Status))
	}
	if sess.AgentSessionStatus != "" {
		ew.F("%s%s\n", lbl("Agent status:"), sess.AgentSessionStatus)
	}
	if sess.TemplateName != "" {
		ew.F("%s%s\n", lbl("Template:"), sess.TemplateName)
	}
	if sess.TemplateID != "" {
		ew.F("%s%s\n", lbl("Template ID:"), s.Slug.Render(sess.TemplateID))
	}
	if sess.TemplateDeleted {
		ew.F("%s%s\n", lbl("Template:"), s.Dim.Render("(deleted)"))
	}
	if sess.TemplateOutdated {
		ew.F("%s%s\n", lbl("Template:"), s.Dim.Render("outdated (template changed since session creation)"))
	}
	if sess.SSHAddress != "" {
		ew.F("%s%s\n", lbl("SSH:"), sess.SSHAddress)
	}
	if sess.VNCAddress != "" {
		ew.F("%s%s\n", lbl("VNC:"), sess.VNCAddress)
	}
	if sess.AutoTerminateAt != nil {
		ew.F("%s%s\n", lbl("Auto-terminates at:"), formatTime(sess.AutoTerminateAt))
	} else if sess.AutoTerminateMinutes > 0 {
		ew.F("%s%d minutes\n", lbl("Auto-terminate:"), sess.AutoTerminateMinutes)
	}
	if sess.CreatedAt != nil {
		ew.F("%s%s\n", lbl("Created:"), formatTime(sess.CreatedAt))
	}
	if sess.UpdatedAt != nil {
		ew.F("%s%s\n", lbl("Updated:"), formatTime(sess.UpdatedAt))
	}
	if snap := sess.TemplateSnapshot; snap != nil {
		if snap.Image != "" || snap.MachineType != "" {
			ew.Ln()
			ew.Ln(s.Dim.Render("Template snapshot"))
			if snap.Image != "" {
				ew.F("%s%s\n", lbl("  Image:"), snap.Image)
			}
			if snap.MachineType != "" {
				ew.F("%s%s\n", lbl("  Machine type:"), snap.MachineType)
			}
			if snap.WorkingDirectory != "" {
				ew.F("%s%s\n", lbl("  Working dir:"), snap.WorkingDirectory)
			}
		}
	}
	return ew.Err
}
