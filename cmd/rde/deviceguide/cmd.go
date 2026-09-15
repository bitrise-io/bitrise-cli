// Package deviceguide wires the `bitrise-cli rde device-guide` command: the
// agent know-how for device sessions (an RDE session that boots an iOS
// simulator or Android emulator — `rde session create --device-platform`).
// The markdown mirrors the RDE backend's device-session guide verbatim; the
// backend is the source of truth — keep these in sync with the backend
// release this CLI targets rather than editing here.
package deviceguide

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
)

//go:embed guides/device-sessions.md
var guideDeviceSessionsRaw string

//go:embed guides/ios.md
var guideIOSRaw string

//go:embed guides/android.md
var guideAndroidRaw string

// The mirror files open with a one-line HTML comment addressed to whoever
// edits them ("Mirror of the RDE device-session guide … do not edit here").
// That note is for the maintainer, not the reader: printed verbatim it was
// the first line every agent read. Strip it here so the file keeps its
// warning and the guide starts at its title.
var (
	guideDeviceSessions = stripMirrorHeader(guideDeviceSessionsRaw)
	guideIOS            = stripMirrorHeader(guideIOSRaw)
	guideAndroid        = stripMirrorHeader(guideAndroidRaw)
)

// stripMirrorHeader drops a leading HTML comment line (and the blank lines
// after it) from a mirrored guide; any other text is returned unchanged.
func stripMirrorHeader(s string) string {
	if !strings.HasPrefix(s, "<!--") {
		return s
	}
	end := strings.Index(s, "-->")
	if end < 0 {
		return s
	}
	return strings.TrimLeft(s[end+len("-->"):], "\r\n")
}

// NewCmd returns the `rde device-guide` command. A leaf command: it prints
// the guide (or a platform's specifics) to stdout, for humans and for agents
// that drive the CLI.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "device-guide [ios|android]",
		Short: "Print the guide for driving a session's iOS simulator / Android emulator",
		Long: `Print the device session guide: how to create a session that boots a virtual
device ('rde session create --device-platform ios|android'), wait for the
device to be ready ('rde session view'), connect, drive it efficiently
(accessibility tree first, then input), let a human watch, and what never to
do. Pass ios or android for that platform's specifics.

The guide is Markdown prose; --output json is rejected (there is no
single-object JSON shape for it).`,
		Example: `  bitrise-cli rde device-guide
  bitrise-cli rde device-guide ios
  bitrise-cli rde device-guide android`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// The inherited --output flag has no JSON shape here: the guide
			// is Markdown prose, not a single object (mirrors `session logs`).
			if cmdutil.ResolveFormat(cmd) == output.JSON {
				return fmt.Errorf("device-guide prints Markdown; --output json is not supported")
			}
			body := guideDeviceSessions
			if len(args) == 1 {
				switch args[0] {
				case "ios":
					body = guideIOS
				case "android":
					body = guideAndroid
				default:
					return fmt.Errorf("unknown platform %q (expected ios or android)", args[0])
				}
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), body)
			return err
		},
	}
}
