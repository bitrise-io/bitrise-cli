package session

import (
	"context"
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

func newCreateCmd() *cobra.Command {
	var (
		description          string
		templateID           string
		stack                string
		machineType          string
		inputs               []string
		secretInputs         []string
		savedInputs          []string
		labels               []string
		featureFlags         []string
		cluster              string
		aiPrompt             string
		autoTerminateMinutes int
		setAutoTerminate     bool
		mapSavedInputs       bool
		wait                 bool
		waitTimeout          time.Duration
		devicePlatform       string
		deviceModel          string
		deviceOSVersion      string
		deviceSystemImage    string
		artifactURL          string
		artifactURLStdin     bool
		artifactName         string
	)

	c := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a new RDE session",
		Long: `Create a new RDE session, either from a template or from a bare
stack + machine type (a template-less session, with no warmup/startup scripts
or other template configuration).

NAME is a human-readable label for the session; you can use it in place of the
session ID in later commands (view, terminate, …) as long as it stays unique.

Pass --template to create the session from a template (by ID or name). To
create a session without a template, omit --template and pass both --stack and
--machine-type instead. --stack / --machine-type may also be given alongside
--template to override the template's defaults for this session.

Provide session input values via --input (one --input per key), --secret-input
(value stored as secret-at-rest), or --saved-input (reference an existing saved
input by ID). Use --map-saved-inputs to auto-fill any session input key that
matches a saved input the user already has.

For secret values, prefer storing them once with 'rde saved-input create
--value-stdin --secret' and referencing them by ID via --saved-input. A value
passed inline with --secret-input ends up in your shell history and in the
process arguments (readable by other users via 'ps'); marking it secret only
governs how the backend stores the value, not how it reaches the CLI.

Attach arbitrary key=value metadata with --label (repeatable); labels come
back on 'session view' and in 'session list --output json', and sessions
can be filtered by them with 'rde session list --label-selector key=value'.

Boot a virtual device with the session by passing --device-platform ios (an
iOS simulator on a macOS stack) or android (an Android emulator on a dockerless
Android Linux stack such as ubuntu-resolute-26.04-bitrise-2026-android; the
Docker-based linux-docker-* stacks are rejected). --stack/--machine-type may
then be omitted, and --cluster is never needed: the deployment's known-good
pair for the platform applies. Optionally pre-install an app with
--artifact-url, or --artifact-url-stdin to read the URL from stdin: a signed
(pre-authenticated) download URL is a bearer credential, and a value passed
inline ends up in your shell history and in the process arguments (readable
by other users via 'ps'). "running" does not mean the device is usable —
'session view' shows the device state; wait for "ready" (--wait does so for
you when a device was requested). Know-how: 'rde device-guide'.

Example values:
  --input key=value
  --saved-input session-key=SAVED_INPUT_ID   # secret stored ahead of time
  --secret-input api-key=VALUE               # inline; avoid for real secrets`,
		Example: `  bitrise-cli rde session create dev --template TEMPLATE_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --input repo=my-app
  # Template-less: pick a stack and machine type directly.
  bitrise-cli rde session create dev --stack osx-xcode-16.0.x-edge --machine-type g2.mac.m2pro.6c-14g
  # Keep secrets off the command line: store once, then reference by ID.
  echo -n "ghp_xxx" | bitrise-cli rde saved-input create --key gh-token --value-stdin --secret
  bitrise-cli rde session create dev --template TEMPLATE_ID --saved-input gh-token=SAVED_INPUT_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --map-saved-inputs
  # Boot an iOS simulator with the session (stack/machine type default to the platform's).
  bitrise-cli rde session create ios-check --device-platform ios --device-model "iPhone 16" --device-os-version 18.2
  bitrise-cli rde session create android-check --device-platform android --artifact-url https://…/app.apk
  # Keep a signed artifact URL out of shell history and process args: read it from a file.
  bitrise-cli rde session create android-check --device-platform android --artifact-url-stdin < artifact-url.txt`,
		Args: cmdutil.RequireArgs("NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if name == "" {
				return fmt.Errorf("NAME must not be empty")
			}
			// A session needs either a template or, for a template-less
			// session, an explicit stack + machine type — unless a device is
			// requested, in which case the backend fills whatever is missing
			// from the platform's defaults. (stack/machine type may also
			// accompany a template to override its defaults.)
			if templateID == "" && devicePlatform == "" && (stack == "" || machineType == "") {
				return fmt.Errorf("provide --template, --device-platform, or both --stack and --machine-type to create a session without a template")
			}
			switch devicePlatform {
			case "", "ios", "android":
			default:
				return fmt.Errorf("--device-platform must be ios or android")
			}
			if devicePlatform == "" && (deviceModel != "" || deviceOSVersion != "" || deviceSystemImage != "" || artifactURL != "" || artifactURLStdin || artifactName != "") {
				return fmt.Errorf("--device-model, --device-os-version, --device-system-image, --artifact-url, --artifact-url-stdin and --artifact-name require --device-platform")
			}
			if artifactURLStdin {
				// Signed download URLs are bearer credentials; reading them
				// from stdin keeps them out of shell history and `ps` (same
				// rationale as `saved-input create --value-stdin`).
				v, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "", true)
				if err != nil {
					return fmt.Errorf("reading --artifact-url-stdin: %w", err)
				}
				if v == "" {
					return fmt.Errorf("--artifact-url-stdin: no URL read from stdin")
				}
				artifactURL = v
			}
			if devicePlatform == "ios" && deviceSystemImage != "" {
				return fmt.Errorf("--device-system-image applies to Android only")
			}
			if artifactName != "" && artifactURL == "" {
				return fmt.Errorf("--artifact-name requires --artifact-url or --artifact-url-stdin")
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			sessionInputs, err := parseSessionInputs(inputs, secretInputs, savedInputs)
			if err != nil {
				return err
			}
			labelMap, err := parseLabelFlags("--label", labels)
			if err != nil {
				return err
			}
			req := internalrde.CreateSessionRequest{
				Name:                    name,
				Description:             description,
				TemplateID:              templateID,
				StackID:                 stack,
				MachineType:             machineType,
				SessionInputs:           sessionInputs,
				EnabledFeatureFlagNames: featureFlags,
				Cluster:                 cluster,
				AIPrompt:                aiPrompt,
				MapSavedToSessionInputs: mapSavedInputs,
				Labels:                  labelMap,
			}
			if setAutoTerminate {
				m := autoTerminateMinutes
				req.AutoTerminateMinutes = &m
			}
			if devicePlatform != "" {
				req.DeviceSpec = &internalrde.DeviceSpec{
					Platform:    devicePlatform,
					DeviceModel: deviceModel,
					OSVersion:   deviceOSVersion,
					SystemImage: deviceSystemImage,
				}
				if artifactURL != "" {
					req.Artifact = &internalrde.DeviceArtifact{URL: artifactURL, AppName: artifactName}
				}
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)

			// --template accepts either a UUID or a template name; resolve
			// names to IDs before issuing CreateSession so the user gets
			// a clean error if the name is wrong or ambiguous. Skipped for
			// template-less sessions, where no template is involved.
			if req.TemplateID != "" {
				resolvedID, err := svc.ResolveTemplateID(cmd.Context(), workspaceID, req.TemplateID)
				if err != nil {
					return err
				}
				req.TemplateID = resolvedID
			}

			res, err := svc.CreateSession(cmd.Context(), workspaceID, req)
			if err != nil {
				return err
			}

			if wait {
				waitCtx, cancel := context.WithTimeout(cmd.Context(), waitTimeout)
				defer cancel()
				if !cmdutil.IsQuiet(cmd) && format != output.JSON {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Waiting for session %s to become ready (timeout %s)…\n", res.Session.ID, waitTimeout)
				}
				ready, waitErr := svc.WaitForReady(waitCtx, workspaceID, res.Session.ID, 0, nil)
				if waitErr != nil {
					return fmt.Errorf("waiting for session: %w", waitErr)
				}
				// A running VM is not a usable device: when one was requested,
				// keep polling (same timeout budget) until it reports ready or failed.
				for ready.Status == "running" && ready.Device != nil && (ready.Device.State == "" || ready.Device.State == "booting") {
					select {
					case <-waitCtx.Done():
						return fmt.Errorf("waiting for device: %w", waitCtx.Err())
					case <-time.After(deviceWaitPollInterval):
					}
					if ready, waitErr = svc.GetSession(waitCtx, workspaceID, res.Session.ID); waitErr != nil {
						return fmt.Errorf("waiting for device: %w", waitErr)
					}
				}
				res.Session = ready
				deviceFailed := ready.Device != nil && ready.Device.State == "failed"
				if ready.Status != "running" || deviceFailed {
					if renderErr := output.Render(cmd.OutOrStdout(), format, res, renderCreateResult); renderErr != nil {
						return renderErr
					}
					cmdutil.SilenceRootErrors(cmd)
					if deviceFailed {
						return fmt.Errorf("session is running but its device failed to boot: %s", ready.Device.DeviceNotes)
					}
					return fmt.Errorf("session ended provisioning with status %q (expected running)", ready.Status)
				}
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderCreateResult)
		},
	}

	c.Flags().StringVar(&description, "description", "", "session description")
	c.Flags().StringVar(&templateID, "template", "", "template ID or name to create the session from (omit to create a template-less session with --stack and --machine-type)")
	c.Flags().StringVar(&stack, "stack", "", "stack ID for a template-less session, or to override the template's stack (see 'rde stack list')")
	c.Flags().StringVar(&machineType, "machine-type", "", "machine type name for a template-less session, or to override the template's machine type (see 'rde machine-type list --stack STACK_ID')")
	c.Flags().StringArrayVar(&inputs, "input", nil, "session input as key=value (repeatable)")
	c.Flags().StringArrayVar(&secretInputs, "secret-input", nil, "session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args — prefer --saved-input)")
	c.Flags().StringArrayVar(&savedInputs, "saved-input", nil, "session input as key=savedInputID — uses a stored saved-input value (repeatable)")
	c.Flags().StringArrayVarP(&labels, "label", "l", nil, "label to attach to the session as key=value (repeatable; at most 32; keys use letters, digits, and . _ / -, values additionally : and +; the bitrise.io/ key prefix is reserved)")
	c.Flags().StringArrayVar(&featureFlags, "feature-flag", nil, "name of a feature flag to enable on the session (repeatable)")
	c.Flags().StringVar(&cluster, "cluster", "", "target cluster name (use 'rde machine-type list --stack STACK_ID' to find candidates when the stack + machine type combo is ambiguous)")
	c.Flags().StringVar(&aiPrompt, "ai-prompt", "", "initial AI prompt passed to Claude Code on session start")
	c.Flags().IntVar(&autoTerminateMinutes, "auto-terminate-minutes", 0, "minutes until auto-termination; 0 disables; omitted uses the backend default (~5 days)")
	c.Flags().BoolVar(&mapSavedInputs, "map-saved-inputs", false, "auto-fill template session inputs from the user's saved inputs (matched by key)")
	c.Flags().StringVar(&devicePlatform, "device-platform", "", "boot a virtual device with the session: ios (simulator, macOS stack) or android (emulator, Linux stack); --stack/--machine-type may then be omitted")
	c.Flags().StringVar(&deviceModel, "device-model", "", "device to boot: simctl device type (\"iPhone 16\") or emulator device profile (\"pixel_7\"); default: platform default")
	c.Flags().StringVar(&deviceOSVersion, "device-os-version", "", "iOS only: iOS version (\"18.2\") or simctl runtime id; default: newest installed")
	c.Flags().StringVar(&deviceSystemImage, "device-system-image", "", "Android only: sdkmanager system image package (\"system-images;android-34;google_apis;x86_64\"); default: platform default")
	c.Flags().StringVar(&artifactURL, "artifact-url", "", "app build to install once the device is ready: absolute http(s) URL of a zipped simulator .app (iOS) or an .apk (Android); requires --device-platform (a signed URL is visible in shell history and process args — prefer --artifact-url-stdin)")
	c.Flags().BoolVar(&artifactURLStdin, "artifact-url-stdin", false, "read the --artifact-url value from stdin instead of the command line; keeps signed URLs out of shell history and process args; requires --device-platform")
	c.Flags().StringVar(&artifactName, "artifact-name", "", "display name of the app installed from --artifact-url / --artifact-url-stdin")
	c.Flags().BoolVar(&wait, "wait", false, "wait until the session leaves provisioning (running, failed, …) — and, with --device-platform, until the device is ready or failed — before returning; exits 1 if the final status isn't running")
	c.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait when --wait is set (uses Go duration syntax: 30s, 5m, 1h)")
	c.MarkFlagsMutuallyExclusive("artifact-url", "artifact-url-stdin")

	c.PreRun = func(cmd *cobra.Command, _ []string) {
		// Track whether --auto-terminate-minutes was explicitly set so we
		// can distinguish "not provided" from "set to 0".
		setAutoTerminate = cmd.Flags().Changed("auto-terminate-minutes")
	}
	return c
}

// deviceWaitPollInterval is how often --wait re-reads a device session while
// its device is still booting (a variable so tests can shorten it).
var deviceWaitPollInterval = 3 * time.Second

// parseSessionInputs converts the user-friendly --input/--secret-input/--saved-input
// flags into SessionInputValue entries. Returns an error on the first malformed
// entry; later iterations don't run.
func parseSessionInputs(plain, secret, saved []string) ([]internalrde.SessionInputValue, error) {
	out := make([]internalrde.SessionInputValue, 0, len(plain)+len(secret)+len(saved))
	for _, kv := range plain {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--input %q: expected key=value", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, Value: v})
	}
	for _, kv := range secret {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--secret-input %q: expected key=value", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, Value: v, IsSecret: true})
	}
	for _, kv := range saved {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("--saved-input %q: expected key=savedInputID", kv)
		}
		out = append(out, internalrde.SessionInputValue{Key: k, SavedInputID: v})
	}
	return out, nil
}

func renderCreateResult(w io.Writer, res internalrde.CreateSessionResult) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	ew.F("%s %s\n", s.BuildStatus("success").Render("✓"), "Session created")
	if err := renderSessionDetail(w, res.Session); err != nil {
		return err
	}
	if len(res.AutoMappedInputs) > 0 {
		ew.Ln()
		ew.Ln(s.Dim.Render("Auto-mapped session inputs from saved inputs:"))
		for _, m := range res.AutoMappedInputs {
			ew.F("  %s → %s\n", m.SessionInputKey, s.Slug.Render(m.SavedInputID))
		}
	}
	return ew.Err
}
