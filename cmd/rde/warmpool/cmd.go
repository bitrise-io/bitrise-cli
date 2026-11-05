// Package warmpool wires `bitrise-cli rde pool` subcommands.
package warmpool

import (
	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
)

// NewCmd returns the `rde pool` parent command.
func NewCmd() *cobra.Command {
	return newGroup("pool", []string{"pools"}, false)
}

// NewHiddenAliasCmd returns the same command group under its former name,
// `rde warm-pool`, hidden from help so existing scripts keep working.
func NewHiddenAliasCmd() *cobra.Command {
	return newGroup("warm-pool", []string{"warm-pools"}, true)
}

func newGroup(use string, aliases []string, hidden bool) *cobra.Command {
	c := &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Hidden:  hidden,
		Short:   "Keep pre-booted sessions ready to claim (pools)",
		Long: `Keep pre-booted sessions ready to claim.

A pool is a stored session configuration — a template, its session
input values and feature flags, and optional stack / machine type / cluster
overrides — plus a warm sessions count, owned by the workspace. The RDE backend keeps that
many sessions of the configuration booted and idle ("warm sessions"), so
'rde session create --warm-pool POOL' hands one out instantly (the session's
warm state is "claimed") instead of booting a VM. When none is available the
session is created from the pool's configuration ("cold"), so a pool with
0 warm sessions still works as a configuration preset.

Every pool belongs to the workspace: every member sees, claims from and
manages it, Workspace API Tokens reach it, and it can back preview links
('rde preview-link create --warm-pool'). Inputs are stored as plain values —
saved inputs are personal and never reach a pool. A session claimed from a
pool is yours; pass '--owner workspace' to 'rde session create' to keep it
the workspace's instead.

On a template with lifecycle hooks a pool's inputs are the Shared and On
create ones (set once, for everyone who claims); each claimant supplies the
On claim and On restore inputs on 'rde session create --warm-pool'. A
value marked secret (--secret-input) is encrypted at rest and hidden in
output. A warm session is ready to claim once
On create finished. When On create fails three times in a row the pool
pauses with the last machine kept for inspection ('view' shows "Setup
failed:"); fix the script, then 'rde pool retry'.

Warm sessions cost machine time while idle. Set the warm sessions to what
demand needs and scale it with 'rde pool set-size POOL N' — the
intended way to warm a pool for business hours is a cron job that sets the
size in the morning and back to 0 in the evening (a Workspace API Token
works). Secret input values are never shown; 'view'
reports each secret key as (hidden).

Commands that take a POOL_ID also accept a pool name — it's resolved to
an ID for you. Names aren't unique, so if more than one pool shares the name
the command errors and lists the candidate IDs to pick from.

'rde warm-pool' is the former name of this command group and keeps working;
the API, the --warm-pool flag and the JSON field warm_pool_id keep the old
spelling too.`,
		Example: `  bitrise-cli rde pool list
  bitrise-cli rde pool create ios-devs --template TEMPLATE_ID --size 2 --secret-input GITHUB_TOKEN=ghp_xxx
  bitrise-cli rde pool set-size ios-devs 0     # drain for the night
  bitrise-cli rde session create dev --warm-pool ios-devs`,
		RunE: cmdutil.DelegateToList,
	}
	c.AddCommand(
		newListCmd(),
		newViewCmd(),
		newCreateCmd(),
		newUpdateCmd(),
		newSetSizeCmd(),
		newRetryCmd(),
		newDeleteCmd(),
	)
	return c
}
