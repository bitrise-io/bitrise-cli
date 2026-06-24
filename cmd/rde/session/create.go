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
		image                string
		machineType          string
		inputs               []string
		secretInputs         []string
		savedInputs          []string
		featureFlags         []string
		cluster              string
		aiPrompt             string
		autoTerminateMinutes int
		setAutoTerminate     bool
		mapSavedInputs       bool
		wait                 bool
		waitTimeout          time.Duration
	)

	c := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a new RDE session",
		Long: `Create a new RDE session, either from a template or from a bare
image + machine type (a template-less session, with no warmup/startup scripts
or other template configuration).

NAME is a human-readable label for the session; you can use it in place of the
session ID in later commands (view, terminate, …) as long as it stays unique.

Pass --template to create the session from a template (by ID or name). To
create a session without a template, omit --template and pass both --image and
--machine-type instead. --image / --machine-type may also be given alongside
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

Example values:
  --input key=value
  --saved-input session-key=SAVED_INPUT_ID   # secret stored ahead of time
  --secret-input api-key=VALUE               # inline; avoid for real secrets`,
		Example: `  bitrise-cli rde session create dev --template TEMPLATE_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --input repo=my-app
  # Template-less: pick an image and machine type directly.
  bitrise-cli rde session create dev --image osx-sequoia-26 --machine-type g2.mac.m2pro.6c-14g
  # Keep secrets off the command line: store once, then reference by ID.
  echo -n "ghp_xxx" | bitrise-cli rde saved-input create --key gh-token --value-stdin --secret
  bitrise-cli rde session create dev --template TEMPLATE_ID --saved-input gh-token=SAVED_INPUT_ID
  bitrise-cli rde session create dev --template TEMPLATE_ID --map-saved-inputs`,
		Args: cmdutil.RequireArgs("NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if name == "" {
				return fmt.Errorf("NAME must not be empty")
			}
			// A session needs either a template or, for a template-less
			// session, an explicit image + machine type. (image/machine type
			// may also accompany a template to override its defaults.)
			if templateID == "" && (image == "" || machineType == "") {
				return fmt.Errorf("provide --template, or both --image and --machine-type to create a session without a template")
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			sessionInputs, err := parseSessionInputs(inputs, secretInputs, savedInputs)
			if err != nil {
				return err
			}
			req := internalrde.CreateSessionRequest{
				Name:                    name,
				Description:             description,
				TemplateID:              templateID,
				Image:                   image,
				MachineType:             machineType,
				SessionInputs:           sessionInputs,
				EnabledFeatureFlagNames: featureFlags,
				Cluster:                 cluster,
				AIPrompt:                aiPrompt,
				MapSavedToSessionInputs: mapSavedInputs,
			}
			if setAutoTerminate {
				m := autoTerminateMinutes
				req.AutoTerminateMinutes = &m
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
				res.Session = ready
				if ready.Status != "running" {
					if renderErr := output.Render(cmd.OutOrStdout(), format, res, renderCreateResult); renderErr != nil {
						return renderErr
					}
					cmdutil.SilenceRootErrors(cmd)
					return fmt.Errorf("session ended provisioning with status %q (expected running)", ready.Status)
				}
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderCreateResult)
		},
	}

	c.Flags().StringVar(&description, "description", "", "session description")
	c.Flags().StringVar(&templateID, "template", "", "template ID or name to create the session from (omit to create a template-less session with --image and --machine-type)")
	c.Flags().StringVar(&image, "image", "", "machine image name for a template-less session, or to override the template's image (see 'rde image list')")
	c.Flags().StringVar(&machineType, "machine-type", "", "machine type name for a template-less session, or to override the template's machine type (see 'rde machine-type list --image NAME')")
	c.Flags().StringArrayVar(&inputs, "input", nil, "session input as key=value (repeatable)")
	c.Flags().StringArrayVar(&secretInputs, "secret-input", nil, "session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args — prefer --saved-input)")
	c.Flags().StringArrayVar(&savedInputs, "saved-input", nil, "session input as key=savedInputID — uses a stored saved-input value (repeatable)")
	c.Flags().StringArrayVar(&featureFlags, "feature-flag", nil, "name of a feature flag to enable on the session (repeatable)")
	c.Flags().StringVar(&cluster, "cluster", "", "target cluster name (use 'rde machine-type list --image NAME' to find candidates when the image + machine type combo is ambiguous)")
	c.Flags().StringVar(&aiPrompt, "ai-prompt", "", "initial AI prompt passed to Claude Code on session start")
	c.Flags().IntVar(&autoTerminateMinutes, "auto-terminate-minutes", 0, "minutes until auto-termination; 0 disables; omitted uses backend default (~5 days)")
	c.Flags().BoolVar(&mapSavedInputs, "map-saved-inputs", false, "auto-fill template session inputs from the user's saved inputs (matched by key)")
	c.Flags().BoolVar(&wait, "wait", false, "wait until the session leaves provisioning (running, failed, …) before returning; exits 1 if the final status isn't running")
	c.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "max time to wait when --wait is set (uses Go duration syntax: 30s, 5m, 1h)")

	c.PreRun = func(cmd *cobra.Command, _ []string) {
		// Track whether --auto-terminate-minutes was explicitly set so we
		// can distinguish "not provided" from "set to 0".
		setAutoTerminate = cmd.Flags().Changed("auto-terminate-minutes")
	}
	return c
}

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
