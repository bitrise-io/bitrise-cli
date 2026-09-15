// Package previewlink wires the `bitrise-cli rde preview-link` commands.
package previewlink

import "github.com/spf13/cobra"

// NewCmd returns the `rde preview-link` parent command.
//
// It has one verb on purpose: preview links are stateless and nothing is
// stored when one is minted, so there is no list, view, update or delete to
// offer. A link is used until it expires, and the sessions it opened are
// ordinary sessions under `rde session`.
func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "preview-link",
		Aliases: []string{"preview-links"},
		Short:   "Create shareable device preview links for app builds",
		Long: `Create a shareable link that opens an app build on a live iOS simulator or
Android emulator in the recipient's browser — no Bitrise login, no local
tooling, nothing for them to install.

Minting is the only operation: nothing is stored when a link is created, so a
link cannot be listed, read back or revoked — it is used until it expires. The
sessions a link opens are ordinary sessions; see 'rde session list'.`,
	}
	c.AddCommand(newCreateCmd())
	return c
}
