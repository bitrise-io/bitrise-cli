// Package usage wires the `bitrise-cli rde usage` command.
package usage

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// NewCmd returns the `rde usage` command. A leaf command, not a noun with
// subcommands: the report is a single summary, not a listable collection.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "usage",
		Short: "Show the workspace's active session and resource usage",
		Long: `Show a point-in-time snapshot of the workspace's active remote dev sessions:
session counts and vCPU/memory totals split by OS, workspace-wide and per user.

This reports sessions currently consuming resources; it is not a historical or
billing-period report. Requires the workspace's billing-view permission
(workspace owners and billing-managing custom roles).`,
		Example: `  bitrise-cli rde usage
  bitrise-cli rde usage --output json | jq '.totals'`,
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
			res, err := internalrde.NewService(client).GetWorkspaceUsage(cmd.Context(), workspaceID)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, res, renderUsage)
		},
	}
}

func renderUsage(w io.Writer, res internalrde.WorkspaceUsage) error {
	total := sumBuckets(res.Totals)
	if total.SessionCount == 0 {
		_, err := fmt.Fprintln(w, "No active sessions.")
		return err
	}

	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-12s", label))
	}
	ew.F("%s%-6d%s\n", lbl("Sessions:"), total.SessionCount, osSplit(res.Totals, func(p internalrde.PlatformUsage) int32 { return p.SessionCount }))
	ew.F("%s%-6d%s\n", lbl("vCPU:"), total.VCPU, osSplit(res.Totals, func(p internalrde.PlatformUsage) int32 { return p.VCPU }))
	ew.F("%s%-6d%s\n", lbl("Memory GB:"), total.MemoryGB, osSplit(res.Totals, func(p internalrde.PlatformUsage) int32 { return p.MemoryGB }))

	if len(res.Users) > 0 {
		ew.F("\n")
		if err := renderUserTable(w, s, res.Users); err != nil {
			return err
		}
	}

	if res.UnknownMachineTypeCount > 0 {
		ew.F("\nNote: %d active session(s) have an unrecognized machine type and contribute 0 to the vCPU/memory totals, so the sums may undercount.\n", res.UnknownMachineTypeCount)
	}
	return ew.Err
}

// sumBuckets folds the linux/macos/unknown buckets into one grand total.
func sumBuckets(t internalrde.UsageTotals) internalrde.PlatformUsage {
	return internalrde.PlatformUsage{
		SessionCount: t.Linux.SessionCount + t.Macos.SessionCount + t.Unknown.SessionCount,
		VCPU:         t.Linux.VCPU + t.Macos.VCPU + t.Unknown.VCPU,
		MemoryGB:     t.Linux.MemoryGB + t.Macos.MemoryGB + t.Unknown.MemoryGB,
	}
}

// osSplit renders the per-OS breakdown of one metric, e.g.
// "(Linux 64, macOS 24)"; the unknown bucket appears only when non-zero.
func osSplit(t internalrde.UsageTotals, metric func(internalrde.PlatformUsage) int32) string {
	out := fmt.Sprintf("(Linux %d, macOS %d", metric(t.Linux), metric(t.Macos))
	if v := metric(t.Unknown); v != 0 {
		out += fmt.Sprintf(", unknown %d", v)
	}
	return out + ")"
}

func renderUserTable(w io.Writer, s style.Styles, users []internalrde.UserUsage) error {
	const colUser = 0
	headers := []string{"USER", "SESSIONS", "LINUX VCPU/GB", "MACOS VCPU/GB"}
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		rows = append(rows, []string{
			userLabel(u),
			strconv.Itoa(int(sumBuckets(u.Totals).SessionCount)),
			vcpuGB(u.Totals.Linux),
			vcpuGB(u.Totals.Macos),
		})
	}
	styler := func(row, col int, content string) string {
		if col == colUser && users[row].IsWorkspace {
			return s.Dim.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}

// userLabel identifies a breakdown row: email when known, the workspace
// bucket as "(workspace)", with username/slug fallbacks for user rows
// missing an email.
func userLabel(u internalrde.UserUsage) string {
	switch {
	case u.IsWorkspace:
		return "(workspace)"
	case u.Email != "":
		return u.Email
	case u.Username != "":
		return u.Username
	default:
		return u.UserSlug
	}
}

func vcpuGB(p internalrde.PlatformUsage) string {
	return fmt.Sprintf("%d / %d", p.VCPU, p.MemoryGB)
}
