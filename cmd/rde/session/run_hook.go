package session

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newRunHookCmd() *cobra.Command {
	var hook string
	c := &cobra.Command{
		Use:   "run-hook SESSION_ID --hook on_claim|on_restore|personalize",
		Short: "Queue a lifecycle hook to run again on a session (keeps the machine)",
		Long: `Queue one lifecycle hook to run again on a session that runs lifecycle hooks
('session view' shows a "Setup:" line). The session's hook runner picks it up
on its next poll and runs the session's creation-time copy of the script with
the session's inputs; 'session view' lists the new attempt (triggered by
"run_again") and 'session logs --stage <hook> --attempt N' shows its output.

Use it after a setup failure ("Setup: setup failed" / "personalization
failed" / "restore script failed") whose cause was transient — a flaky
download, a service that was not up yet — because the machine and everything
on it stay as they are. A fix to the template's script does not reach an
existing session (it keeps its creation-time hooks): 'session rebuild' or a
new session is the way to pick it up.

  --hook   on_claim or on_restore (only hooks the session was created
           with), or personalize — the platform's own System setup step
           (credentials, Claude Code) — on a session that already ran it.

On create is not accepted: it runs once per machine, so a fresh attempt
needs a fresh machine — 'session rebuild' discards the machine and runs the
lifecycle again from On create ("On create cannot be run again on this
machine. Use Rebuild to run it on a fresh one.").

The backend refuses the request while a run is already pending or running on
the session (wait for the setup to settle), on a warm pool's unclaimed
sessions, on a session without lifecycle hooks and on a session without a
machine (restore it first).`,
		Example: `  bitrise-cli rde session run-hook SESSION_ID --hook on_restore
  bitrise-cli rde session run-hook SESSION_ID --hook personalize --output json`,
		Args: cmdutil.RequireArgs("SESSION_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch hook {
			case internalrde.HookOnClaim, internalrde.HookOnRestore, internalrde.HookPersonalize:
			case internalrde.HookOnCreate:
				// The backend's own words (FailedPrecondition), with the
				// command that does what "Rebuild" means here.
				return fmt.Errorf("%s ('bitrise-cli rde session rebuild %s')", internalrde.ErrOnCreateNotRunnable, args[0])
			default:
				return fmt.Errorf("--hook must be %s, %s or %s (On create cannot be run again: use 'session rebuild')", internalrde.HookOnClaim, internalrde.HookOnRestore, internalrde.HookPersonalize)
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			sessionID, err := svc.ResolveSessionID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			run, err := svc.RunSessionHook(cmd.Context(), workspaceID, sessionID, hook)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), cmdutil.ResolveFormat(cmd), run, renderQueuedHookRun)
		},
	}
	c.Flags().StringVar(&hook, "hook", "", "which hook to run again: on_claim, on_restore or personalize (required); On create cannot be run again — use 'session rebuild'")
	_ = c.MarkFlagRequired("hook")
	_ = c.RegisterFlagCompletionFunc("hook", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			internalrde.HookOnClaim + "\tthe On claim hook",
			internalrde.HookOnRestore + "\tthe On restore hook",
			internalrde.HookPersonalize + "\tthe platform's System setup step",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	return c
}

// renderQueuedHookRun prints the run 'run-hook' queued.
func renderQueuedHookRun(w io.Writer, run internalrde.HookRun) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	ew.F("%s #%d queued (%s)\n", hookLabel(run.Hook), run.Attempt, run.State)
	ew.Ln(s.Dim.Render("The session's hook runner starts it on its next poll; follow it with 'rde session view' or 'rde session logs --stage <hook> --attempt " + fmt.Sprint(run.Attempt) + "'."))
	return ew.Err
}
