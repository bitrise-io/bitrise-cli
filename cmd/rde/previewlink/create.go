package previewlink

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// maxTTL mirrors the backend's cap. Checked here only to fail a CI step
// before the round trip; the backend is the authority and its message wins
// for anything this misses.
const maxTTL = 72 * time.Hour

// minAutoTerminateMinutes and maxAutoTerminateMinutes mirror the backend's
// bounds on an explicit idle window; the ceiling is the 8-hour session max
// lifetime, which bounds a device's total runtime either way. 0 still means
// "the default", not "off": auto-terminate is the cost backstop on a link
// anyone can open, and cannot be disabled.
const (
	minAutoTerminateMinutes = 10
	maxAutoTerminateMinutes = 480
)

func newCreateCmd() *cobra.Command {
	var (
		devicePlatform    string
		deviceModel       string
		deviceOSVersion   string
		deviceSystemImage string
		artifactURL       string
		artifactURLStdin  bool
		artifactName      string
		artifactBuild     string
		artifactCommit    string
		ttl               time.Duration
		autoTerminateMins int
		stack             string
		machineType       string
		warmPool          string
	)

	c := &cobra.Command{
		Use:   "create",
		Short: "Mint a shareable device preview link for an app build",
		Long: `Mint a shareable link that opens an app build on a live iOS simulator or
Android emulator in the recipient's browser — no Bitrise login, no local
tooling, nothing for them to install. The usual shape is a CI step that builds
a simulator/emulator app and posts the link on the pull request.

The app build (--artifact-url) must be reachable by a plain anonymous GET — a
presigned URL is fine, and it is never shown to viewers. It must be a
SIMULATOR/EMULATOR build, not a device build: iOS wants a zipped .app carrying
an arm64 simulator slice, Android an .apk that runs on x86_64. A device build
installs on neither, and the viewer only finds out after a device has booted.

The link is a bearer credential. Anyone holding the URL can open a device on
this workspace's bill, and there is no way to revoke one — a short --ttl is the
control. Post it on a pull request or a team channel, never a public one.

Nothing is stored when the link is minted: this output is the only record that
will exist, so capture the URL. A link nobody opens costs nothing, and every
open gets its own device, so one link serves several reviewers at once.

Limits: --ttl defaults to 24 hours and is capped at 72. At most 5 devices alive
per link and 20 per workspace; opens past the cap are refused. Each device
auto-terminates after its idle window (--auto-terminate-minutes, at least 10,
default 60).

In CI, authenticate with a Workspace API Token rather than a personal one:
minting preview links is one of the few RDE operations a workspace token may
perform, and the sessions it opens belong to the workspace instead of a person.
Pass it as BITRISE_TOKEN, which is used verbatim, and name the workspace with
--workspace or BITRISE_WORKSPACE_ID — a workspace token belongs to one
workspace and cannot look up which workspaces an account has, so leaving it to
be auto-detected fails.

Warm pools: pass --warm-pool to serve the link's opens from a workspace-owned
warm pool ('rde warm-pool list'). Each open claims one of the pool's
pre-booted device sessions when one is available — the click-to-app time is
the app download, not a VM boot — and creates a session from the pool's
configuration otherwise. The pool fixes the device and the machine, so omit
--device-platform and the other --device-* flags, --stack and --machine-type
with it; the pool's configuration must boot a device. A pool deleted after
minting degrades later opens to the ordinary cold path.
`,
		Example: `  bitrise-cli rde preview-link create --device-platform android --artifact-url https://…/app.apk
  # Serve opens from a warm pool's pre-booted devices (the pool fixes the device).
  bitrise-cli rde preview-link create --warm-pool WARM_POOL_ID --artifact-url https://…/app.apk
  bitrise-cli rde preview-link create --device-platform ios --artifact-url https://…/App.zip --device-model "iPhone 16"
  bitrise-cli rde preview-link create --device-platform ios --artifact-url-stdin < artifact-url.txt
  bitrise-cli rde preview-link create --device-platform android --artifact-url https://…/app.apk --ttl 4h
  bitrise-cli rde preview-link create --device-platform android --artifact-url https://…/app.apk --output json | jq -r .url

  # From CI, with a Workspace API Token (--workspace is required with one):
  BITRISE_TOKEN=bitwat_… bitrise-cli rde preview-link create --workspace WORKSPACE_ID \
    --device-platform ios --artifact-url https://…/App.zip`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if warmPool != "" {
				// The pool fixes the device and the machine; the backend
				// rejects anything that would re-specify them, so name the
				// offending flags here instead of relaying a 400.
				var conflicting []string
				for _, f := range []string{"device-platform", "device-model", "device-os-version", "device-system-image", "stack", "machine-type"} {
					if cmd.Flags().Changed(f) {
						conflicting = append(conflicting, "--"+f)
					}
				}
				if len(conflicting) > 0 {
					return fmt.Errorf("--warm-pool: the pool fixes the device and the machine, so %s cannot be combined with it", strings.Join(conflicting, ", "))
				}
			} else {
				if devicePlatform != "ios" && devicePlatform != "android" {
					return fmt.Errorf("--device-platform must be ios or android (or pass --warm-pool to take the pool's device)")
				}
				if devicePlatform == "ios" && deviceSystemImage != "" {
					return fmt.Errorf("--device-system-image applies to Android only")
				}
			}
			if artifactURLStdin {
				// Signed download URLs are bearer credentials; reading them
				// from stdin keeps them out of shell history and `ps` (same
				// rationale as `session create --artifact-url-stdin`).
				v, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "", true)
				if err != nil {
					return fmt.Errorf("reading --artifact-url-stdin: %w", err)
				}
				if v == "" {
					return fmt.Errorf("--artifact-url-stdin: no URL read from stdin")
				}
				artifactURL = v
			}
			if artifactURL == "" {
				return fmt.Errorf("--artifact-url or --artifact-url-stdin is required: a preview link exists to show an app build")
			}
			if autoTerminateMins < 0 {
				return fmt.Errorf("--auto-terminate-minutes must not be negative")
			}
			if autoTerminateMins > 0 && autoTerminateMins < minAutoTerminateMinutes {
				return fmt.Errorf("--auto-terminate-minutes must be at least %d when set — a shorter window would end devices before they finish booting", minAutoTerminateMinutes)
			}
			if autoTerminateMins > maxAutoTerminateMinutes {
				return fmt.Errorf("--auto-terminate-minutes must be at most %d — a device never outlives the 8-hour session cap anyway", maxAutoTerminateMinutes)
			}
			if ttl < 0 {
				return fmt.Errorf("--ttl must not be negative")
			}
			if ttl > maxTTL {
				return fmt.Errorf("--ttl must be at most %s — a short lifetime is the only control on a link that cannot be revoked", maxTTL)
			}

			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}

			req := internalrde.CreatePreviewLinkRequest{
				Artifact: &internalrde.DeviceArtifact{
					URL:         artifactURL,
					AppName:     artifactName,
					BuildNumber: artifactBuild,
					CommitSHA:   artifactCommit,
				},
				TTL:                         ttl,
				StackID:                     stack,
				MachineType:                 machineType,
				SessionAutoTerminateMinutes: autoTerminateMins,
			}
			if devicePlatform != "" {
				// Absent with --warm-pool: the pool's device applies.
				req.DeviceSpec = &internalrde.DeviceSpec{
					Platform:    devicePlatform,
					DeviceModel: deviceModel,
					OSVersion:   deviceOSVersion,
					SystemImage: deviceSystemImage,
				}
			}
			svc := internalrde.NewService(client)
			if warmPool != "" {
				// --warm-pool takes a UUID or a pool name, like session create.
				req.WarmPoolID, err = svc.ResolveWarmPoolID(cmd.Context(), workspaceID, warmPool)
				if err != nil {
					return err
				}
			}
			link, err := svc.CreatePreviewLink(cmd.Context(), workspaceID, req)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, link, renderLink)
		},
	}

	c.Flags().StringVar(&devicePlatform, "device-platform", "", "device each open boots: ios (simulator) or android (emulator) (required)")
	c.Flags().StringVar(&deviceModel, "device-model", "", "device to boot: simctl device type (\"iPhone 16\") or emulator device profile (\"pixel_7\") — a screen profile, not that phone's firmware; default: the platform default")
	c.Flags().StringVar(&deviceOSVersion, "device-os-version", "", "iOS only: an iOS version (\"18.2\") or simctl runtime id — anything else is rejected; default: newest installed")
	c.Flags().StringVar(&deviceSystemImage, "device-system-image", "", "Android only: the API-level knob — sdkmanager system image package (\"system-images;android-34;google_apis;x86_64\"); default: the platform default")
	c.Flags().StringVar(&artifactURL, "artifact-url", "", "app build to install on every open: absolute http(s) URL of a zipped simulator .app (iOS) or an .apk (Android), fetchable by anonymous GET (a signed URL is visible in shell history and process args — prefer --artifact-url-stdin)")
	c.Flags().BoolVar(&artifactURLStdin, "artifact-url-stdin", false, "read the --artifact-url value from stdin instead of the command line; keeps signed URLs out of shell history and process args")
	c.Flags().StringVar(&artifactName, "artifact-name", "", "app display name shown on the viewer page")
	c.Flags().StringVar(&artifactBuild, "artifact-build-number", "", "build number shown on the viewer page")
	c.Flags().StringVar(&artifactCommit, "artifact-commit", "", "commit SHA shown on the viewer page")
	c.Flags().DurationVar(&ttl, "ttl", 0, "how long the link stays openable (Go duration syntax: 4h, 30m); 0 uses the default of 24h, maximum 72h")
	c.Flags().IntVar(&autoTerminateMins, "auto-terminate-minutes", 0, "minutes a device stays alive after its last viewer disconnects; 0 uses the default of 60; minimum 10, maximum 480")
	c.Flags().StringVar(&stack, "stack", "", "stack the link's devices run on; omit for the platform default (see 'rde stack list')")
	c.Flags().StringVar(&machineType, "machine-type", "", "machine type the link's devices run on; omit for the platform default (see 'rde machine-type list --stack STACK_ID')")
	c.Flags().StringVar(&warmPool, "warm-pool", "", "workspace-owned warm pool (ID or name) whose pre-booted device sessions serve the link's opens; the pool fixes the device and the machine, so omit the --device-* flags, --stack and --machine-type (see 'rde warm-pool list')")
	c.MarkFlagsMutuallyExclusive("artifact-url", "artifact-url-stdin")
	return c
}

func renderLink(w io.Writer, link internalrde.PreviewLink) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string { return s.Label.Render(fmt.Sprintf("%-12s", label)) }

	if link.URL != "" {
		ew.F("%s%s\n", lbl("Link:"), link.URL)
	} else {
		// No viewer URL came back: the token is all the caller gets, and
		// they compose their own URL from it.
		ew.F("%s%s\n", lbl("Token:"), link.Token)
	}
	if link.ExpiresAt != nil {
		ew.F("%s%s\n", lbl("Expires:"), link.ExpiresAt.Format(time.RFC3339))
	}
	ew.F("%s%s\n", lbl("Link ID:"), link.JTI)
	ew.F("\n%s anyone with this link can open a device — it cannot be revoked, only left to expire.\n",
		s.Warn.Render("Note:"))
	return ew.Err
}
