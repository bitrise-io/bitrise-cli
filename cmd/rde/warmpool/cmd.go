// Package warmpool wires `bitrise-cli rde warm-pool` subcommands.
package warmpool

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `rde warm-pool` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "warm-pool",
		Aliases: []string{"warm-pools"},
		Short:   "Keep pre-booted sessions ready to claim (warm pools)",
		Long: `Keep pre-booted sessions ready to claim.

A warm pool is a stored session configuration — a template, its session
input values and feature flags, and optional stack / machine type / cluster
overrides — plus a pool size, owned by the workspace. The RDE backend keeps that
many sessions of the configuration booted and idle ("warm sessions"), so
'rde session create --warm-pool POOL' hands one out instantly (the session's
warm state is "claimed") instead of booting a VM. When none is available the
session is created from the pool's configuration ("cold"), so a pool with a
pool size of 0 still works as a configuration preset.

Every pool belongs to the workspace: every member sees, claims from and
manages it, Workspace API Tokens reach it, and it can back preview links
('rde preview-link create --warm-pool'). Inputs are stored as plain values —
saved inputs are personal and never reach a pool. A session claimed from a
pool is yours; pass '--owner workspace' to 'rde session create' to keep it
the workspace's instead.

Warm sessions cost machine time while idle. Set the pool size to what
demand needs and scale it with 'rde warm-pool set-size POOL N' — the
intended way to warm a pool for business hours is a cron job that sets the
size in the morning and back to 0 in the evening (a Workspace API Token
works). Secret input values are never shown; 'view'
reports each secret key as (hidden).

Commands that take a WARM_POOL_ID also accept a pool name — it's resolved to
an ID for you. Names aren't unique, so if more than one pool shares the name
the command errors and lists the candidate IDs to pick from.`,
		Example: `  bitrise-cli rde warm-pool list
  bitrise-cli rde warm-pool create ios-devs --template TEMPLATE_ID --size 2 --secret-input GITHUB_TOKEN=ghp_xxx
  bitrise-cli rde warm-pool set-size ios-devs 0     # drain for the night
  bitrise-cli rde session create dev --warm-pool ios-devs`,
		RunE: cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newSetSizeCmd(),
		newDeleteCmd(),
	)
	return c
}
