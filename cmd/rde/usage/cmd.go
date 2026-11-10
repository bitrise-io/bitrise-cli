// Package usage wires the `bitrise-cli rde usage` command.
package usage

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

// NewCmd returns the `rde usage` command. A leaf command, not a noun with
// subcommands: the report is a single summary, not a listable collection.
func NewCmd() *cobra.Command {
	var listSessions bool
	c := &cobra.Command{
		Use:   "usage",
		Short: "Show the workspace's active session and resource usage",
		Long: `Show a point-in-time snapshot of the workspace's active remote dev sessions:
session counts and vCPU/memory totals split by OS, workspace-wide and per user.

This reports sessions currently consuming resources; it is not a historical or
billing-period report. Requires the workspace's billing-view permission
(workspace owners and billing-managing custom roles).

It also shows the workspace's vCPU quota per machine family the plan includes:
the vCPUs in use (warm pool sessions included) out of the plan's quota. A new session cannot
start on a family once its vCPUs would push the use over the quota.

Pass --sessions to also list, under each user, the active sessions behind the
numbers, oldest first: name, status, age, OS, vCPU/memory and ID. With
--output json the list is always there, as users[].sessions. The list is
filled only on a workspace where terminating any session is enabled (it is
enabled per workspace on request); elsewhere it is empty and the report is
otherwise the same.

On such a workspace, whoever can view this report can also terminate any
listed session with 'rde session terminate SESSION_ID' — a way to find and
stop sessions left running. The exception is a session held by a pool (HELD BY
POOL in the list, held_by_warm_pool_id in JSON): a warm session nobody has
claimed yet. Lower the pool's size to remove it ('rde warm-pool set-size').`,
		Example: `  bitrise-cli rde usage
  bitrise-cli rde usage --sessions
  bitrise-cli rde usage --output json | jq '.totals'
  bitrise-cli rde usage --output json | jq '.compute_quotas'
  # IDs of the sessions created more than 7 days ago, pool-held ones left out
  bitrise-cli rde usage --output json | jq -r '.users[].sessions[]
    | select(.held_by_warm_pool_id == "" and .created_at < (now - 7*86400 | todate)) | .id'`,
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
			render := func(w io.Writer, res internalrde.WorkspaceUsage) error {
				return renderUsage(w, res, listSessions, time.Now())
			}
			if err := output.Render(cmd.OutOrStdout(), format, res, render); err != nil {
				return err
			}
			// Diagnostics go to stderr so piped stdout stays parse-safe; the
			// JSON shape carries unknown_machine_type_count as data anyway.
			if res.UnknownMachineTypeCount > 0 {
				s := style.New(cmd.ErrOrStderr())
				ew := cmdutil.NewErrWriter(cmd.ErrOrStderr())
				ew.F("%s active sessions with unrecognized machine types contribute 0 vCPU/memory to the totals, so the sums may undercount (%d affected)\n",
					s.Warn.Render("Warning:"), res.UnknownMachineTypeCount)
				return ew.Err
			}
			return nil
		},
	}
	c.Flags().BoolVar(&listSessions, "sessions", false, "also list each user's active sessions, oldest first (--output json always includes them)")
	return c
}

// renderUsage prints the report. listSessions adds the per-user session
// lists between the per-user table and the quota table; now is the instant
// the sessions' ages are measured against.
func renderUsage(w io.Writer, res internalrde.WorkspaceUsage, listSessions bool, now time.Time) error {
	s := style.New(w)
	if err := renderSessions(w, s, res); err != nil {
		return err
	}
	if listSessions {
		if err := renderUserSessions(w, s, res, now); err != nil {
			return err
		}
	}
	ew := cmdutil.NewErrWriter(w)
	ew.F("\n")
	if ew.Err != nil {
		return ew.Err
	}
	return renderQuotas(w, s, res.ComputeQuotas)
}

// renderSessions prints the active-session totals and the per-user table.
func renderSessions(w io.Writer, s style.Styles, res internalrde.WorkspaceUsage) error {
	total := sumBuckets(res.Totals)
	if total.SessionCount == 0 {
		_, err := fmt.Fprintln(w, "No active sessions.")
		return err
	}

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
	return ew.Err
}

// renderUserSessions prints, under each user, the active sessions behind the
// row's totals, oldest first (the order the backend sends). A report with
// active sessions but no session on any row comes from a workspace where the
// list is not enabled: that gets one plain sentence instead of nothing.
func renderUserSessions(w io.Writer, s style.Styles, res internalrde.WorkspaceUsage, now time.Time) error {
	if sumBuckets(res.Totals).SessionCount == 0 {
		// "No active sessions." already says it.
		return nil
	}
	ew := cmdutil.NewErrWriter(w)
	listed, held := false, false
	for _, u := range res.Users {
		if len(u.Sessions) == 0 {
			continue
		}
		listed = true
		ew.F("\n%s\n", s.Label.Render(userLabel(u)))
		if ew.Err != nil {
			return ew.Err
		}
		tableHeld, err := renderSessionTable(w, s, u.Sessions, now)
		if err != nil {
			return err
		}
		held = held || tableHeld
	}
	switch {
	case !listed:
		ew.F("\nSession list not available: it is not enabled for this workspace (it is enabled per workspace on request).\n")
	case held:
		ew.F("\n%s\n", s.Dim.Render("A session held by a pool cannot be terminated directly: lower the pool's size ('rde warm-pool set-size'), or retry the pool if its setup failed ('rde warm-pool retry')."))
	}
	return ew.Err
}

// renderSessionTable prints one user's sessions and reports whether any of
// them is held by a pool. The HELD BY POOL column (the pool's ID) appears
// only when some row needs it, like the unknown-OS column of the user table.
func renderSessionTable(w io.Writer, s style.Styles, sessions []internalrde.UsageSession, now time.Time) (bool, error) {
	held := false
	for _, sess := range sessions {
		if sess.HeldByWarmPoolID != "" {
			held = true
			break
		}
	}
	headers := []string{"NAME", "STATUS", "AGE", "OS", "VCPU/GB"}
	if held {
		headers = append(headers, "HELD BY POOL")
	}
	headers = append(headers, "ID")
	const colStatus = 1
	colPool, colID := -1, len(headers)-1
	if held {
		colPool = len(headers) - 2
	}
	rows := make([][]string, 0, len(sessions))
	for _, sess := range sessions {
		row := []string{
			sess.Name,
			sess.Status,
			formatAge(sess.CreatedAt, now),
			osLabel(sess.OS),
			fmt.Sprintf("%d / %d", sess.VCPU, sess.MemoryGB),
		}
		if held {
			row = append(row, sess.HeldByWarmPoolID)
		}
		rows = append(rows, append(row, sess.ID))
	}
	styler := func(row, col int, content string) string {
		switch col {
		case colStatus:
			if sessions[row].Status == "running" {
				return s.BuildStatus("success").Render(content)
			}
			return s.BuildStatus("in-progress").Render(content)
		case colPool, colID:
			return s.Slug.Render(content)
		}
		return content
	}
	return held, style.Table(w, headers, rows, s.Header, styler)
}

// formatAge renders how long ago a session was created, in its two largest
// units, the smaller one dropped when zero: "3d 4h", "2d", "5h 12m", "42m",
// "<1m". "-" when the creation time is unknown.
func formatAge(created *time.Time, now time.Time) string {
	if created == nil {
		return "-"
	}
	d := now.Sub(*created)
	if d < time.Minute {
		return "<1m"
	}
	minutes := int(d.Minutes())
	days, hours, mins := minutes/(24*60), minutes/60%24, minutes%60
	var major, minor string
	switch {
	case days > 0:
		major = fmt.Sprintf("%dd", days)
		if hours > 0 {
			minor = fmt.Sprintf(" %dh", hours)
		}
	case hours > 0:
		major = fmt.Sprintf("%dh", hours)
		if mins > 0 {
			minor = fmt.Sprintf(" %dm", mins)
		}
	default:
		major = fmt.Sprintf("%dm", mins)
	}
	return major + minor
}

// renderQuotas prints the vCPU quota per machine family. Shown even with no
// active sessions: warm pool machines hold quota too, and the quota itself is
// what a user checks before starting a session.
func renderQuotas(w io.Writer, s style.Styles, quotas []internalrde.ComputeQuota) error {
	if len(quotas) == 0 {
		_, err := fmt.Fprintln(w, s.Dim.Render("vCPU quota: could not be loaded."))
		return err
	}
	const colStatus = 3
	headers := []string{"MACHINE FAMILY", "OS", "VCPU USED / QUOTA", "STATUS"}
	rows := make([][]string, 0, len(quotas))
	for _, q := range quotas {
		rows = append(rows, []string{
			q.DisplayName,
			osLabel(q.OS),
			fmt.Sprintf("%d / %d", q.UsedVCPU, q.QuotaVCPU),
			quotaStatus(q),
		})
	}
	styler := func(row, col int, content string) string {
		if col != colStatus {
			return content
		}
		if quotas[row].UsedVCPU >= quotas[row].QuotaVCPU {
			return s.Warn.Render(content)
		}
		return content
	}
	return style.Table(w, headers, rows, s.Header, styler)
}

// osLabel renders a quota's OS the way the session totals name it; "-" when
// the backend could not tell.
func osLabel(os string) string {
	switch os {
	case "linux":
		return "Linux"
	case "macos":
		return "macOS"
	default:
		return "-"
	}
}

// quotaStatus names the state of one family's quota: "full" once no vCPU is
// left, otherwise the vCPUs left.
func quotaStatus(q internalrde.ComputeQuota) string {
	if q.UsedVCPU >= q.QuotaVCPU {
		return "full"
	}
	return fmt.Sprintf("%d left", q.QuotaVCPU-q.UsedVCPU)
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
	// The unknown-OS column appears only when some row needs it, so the
	// common all-known case stays narrow — but when present it lets every
	// row's session total reconcile with its per-OS resources.
	showUnknown := false
	for _, u := range users {
		if u.Totals.Unknown != (internalrde.PlatformUsage{}) {
			showUnknown = true
			break
		}
	}
	headers := []string{"USER", "SESSIONS", "LINUX VCPU/GB", "MACOS VCPU/GB"}
	if showUnknown {
		headers = append(headers, "UNKNOWN VCPU/GB")
	}
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		row := []string{
			userLabel(u),
			strconv.Itoa(int(sumBuckets(u.Totals).SessionCount)),
			vcpuGB(u.Totals.Linux),
			vcpuGB(u.Totals.Macos),
		}
		if showUnknown {
			row = append(row, vcpuGB(u.Totals.Unknown))
		}
		rows = append(rows, row)
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
// bucket as "(workspace)", with username/ID fallbacks for user rows
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
		return u.UserID
	}
}

func vcpuGB(p internalrde.PlatformUsage) string {
	return fmt.Sprintf("%d / %d", p.VCPU, p.MemoryGB)
}
