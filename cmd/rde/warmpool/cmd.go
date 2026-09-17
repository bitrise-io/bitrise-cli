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
overrides — plus an owner and a desired count. The RDE backend keeps that
many sessions of the configuration booted and idle ("warm sessions"), so
'rde session create --warm-pool POOL' hands one out instantly (the session's
warm state is "claimed") instead of booting a VM. When none is available the
session is created from the pool's configuration ("cold"), so a pool with a
desired count of 0 still works as a configuration preset.

Owner: a "user" pool is private to its creator and may reference the
creator's saved inputs; a "workspace" pool is shared with every member,
reachable by Workspace API Tokens, and stores every input as a plain value.
Only workspace pools can back preview links ('rde preview-link create
--warm-pool').

Warm sessions cost machine time while idle. Set the desired count to what
demand needs and scale it with 'rde warm-pool set-count POOL N' — the
intended way to warm a pool for business hours is a cron job that sets the
count in the morning and back to 0 in the evening (a Workspace API Token
works for workspace pools). Secret input values are never shown; 'view'
reports each secret key as (hidden).

Commands that take a WARM_POOL_ID also accept a pool name — it's resolved to
an ID for you. Names aren't unique, so if more than one pool shares the name
the command errors and lists the candidate IDs to pick from.`,
		Example: `  bitrise-cli rde warm-pool list
  bitrise-cli rde warm-pool create ios-devs --template TEMPLATE_ID --count 2 --owner workspace --secret-input GITHUB_TOKEN=ghp_xxx
  bitrise-cli rde warm-pool set-count ios-devs 0     # drain for the night
  bitrise-cli rde session create dev --warm-pool ios-devs`,
		RunE: cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newSetCountCmd(),
		newDeleteCmd(),
	)
	return c
}
