// Package user holds the cobra commands under `bitrise-cli user`.
package user

import (
	"github.com/spf13/cobra"
)

// NewCmd returns the `bitrise-cli user` parent command.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "user",
		Short: "Create and manage your Bitrise account",
		Long: `Manage your own Bitrise account from the CLI.

Today this surface is limited to account creation. After running
'user create' you must click the link emailed to you, then run
'bitrise-cli auth login --email <addr>' to mint and store an access token.`,
	}
	c.AddCommand(newCreateCmd())
	return c
}
