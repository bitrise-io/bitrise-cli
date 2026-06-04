package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

func newLogsCmd() *cobra.Command {
	var (
		stage         string
		follow        bool
		retryInterval time.Duration
	)

	c := &cobra.Command{
		Use:   "logs SESSION_ID --stage warmup|startup",
		Short: "Print a session's warmup or startup logs",
		Long: `Print the warmup or startup script logs for a session — useful for debugging a
session stuck provisioning or one that came up failed. The stream replays the
whole stage log from the start on every connect.

Note: the backend does not currently signal end-of-log, so the command keeps
running — even after the script has finished — until you stop it with Ctrl-C.
This applies to both modes; redirect or pipe stdout and Ctrl-C once output
stops.

  --stage    which script's logs to show: warmup or startup (required). warmup
             runs once at session creation; startup runs on every session
             start/restart.
  --follow   if the stage hasn't produced any logs yet, wait for it to start
             rather than erroring. Without --follow the command errors right
             away when logs aren't available yet.

--output is ignored — logs stream as raw text. Pipe or redirect as needed;
diagnostics go to stderr so a redirect captures only log text. --output json
is rejected (the feed is plain text, not a single object).`,
		Example: `  bitrise-cli rde session logs SESSION_ID --stage startup
  bitrise-cli rde session logs SESSION_ID --stage warmup
  bitrise-cli rde session logs SESSION_ID --stage startup --follow
  bitrise-cli rde session logs SESSION_ID --stage startup > session.log`,
		Args: cmdutil.RequireArgs("SESSION_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject the unsupported flag combo before any network round-trip.
			if cmdutil.ResolveFormat(cmd) == output.JSON {
				return fmt.Errorf("--output json is not supported for logs (the feed is plain text, not a single object)")
			}
			// Validate --stage up front so a typo errors before the stream
			// header prints (mirrors notifications' --order validation).
			switch stage {
			case internalrde.LogStageWarmup, internalrde.LogStageStartup:
			default:
				return fmt.Errorf("--stage must be %s or %s", internalrde.LogStageWarmup, internalrde.LogStageStartup)
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

			// Stop cleanly on Ctrl-C: cancelling the context ends the stream
			// and StreamSessionLogs returns nil, so the command exits 0.
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			out := cmd.OutOrStdout()
			emit := func(chunk string) error {
				_, werr := io.WriteString(out, chunk)
				return werr
			}

			if follow {
				return runFollow(ctx, cmd, svc, workspaceID, sessionID, stage, retryInterval, emit)
			}
			return runSnapshot(ctx, cmd, svc, workspaceID, sessionID, stage, emit)
		},
	}

	c.Flags().StringVar(&stage, "stage", "", "which logs to show: warmup or startup (required)")
	c.Flags().BoolVarP(&follow, "follow", "f", false, "keep streaming until Ctrl-C, waiting for the stage to start if needed")
	c.Flags().DurationVar(&retryInterval, "retry-interval", 3*time.Second, "poll interval while waiting for the stage to start (only with --follow)")
	_ = c.MarkFlagRequired("stage")

	_ = c.RegisterFlagCompletionFunc("stage", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{
			internalrde.LogStageWarmup + "\twarmup script (once at creation)",
			internalrde.LogStageStartup + "\tstartup script (every start/restart)",
		}, cobra.ShellCompDirectiveNoFileComp
	})
	return c
}

// runSnapshot streams the stage log once. It replays the whole stage log from
// the start; the backend does not currently send EOF when the stage finishes,
// so this returns only when the stream actually closes (session gone) or the
// user hits Ctrl-C. A pre-stream 404 ("logs not available yet") is turned into
// a friendly, actionable message and a non-zero exit rather than a raw API
// error.
func runSnapshot(ctx context.Context, cmd *cobra.Command, svc *internalrde.Service, workspaceID, sessionID, stage string, emit func(string) error) error {
	err := svc.StreamSessionLogs(ctx, workspaceID, sessionID, stage, emit)
	if isLogsNotReady(err) {
		cmdutil.SilenceRootErrors(cmd)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"No %s logs available yet — the session may still be provisioning. Retry shortly, or pass --follow to wait.\n", stage)
		return err
	}
	return err
}

// runFollow streams the stage log live until Ctrl-C or EOF. If the stage
// hasn't started producing logs yet (404), it waits retryInterval and retries
// — the dashboard does the same. The retry is safe because a 404 only ever
// comes back pre-stream (nothing has been printed yet), so it can't duplicate
// already-streamed output.
func runFollow(ctx context.Context, cmd *cobra.Command, svc *internalrde.Service, workspaceID, sessionID, stage string, retryInterval time.Duration, emit func(string) error) error {
	if retryInterval <= 0 {
		retryInterval = 3 * time.Second
	}
	if !cmdutil.IsQuiet(cmd) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Streaming %s logs for session %s — Ctrl-C to stop…\n", stage, sessionID)
	}
	for {
		err := svc.StreamSessionLogs(ctx, workspaceID, sessionID, stage, emit)
		if err == nil {
			return nil
		}
		if isLogsNotReady(err) && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryInterval):
				continue
			}
		}
		return err
	}
}

// isLogsNotReady reports whether err is the backend's pre-stream "logs not
// available yet for this stage" 404.
func isLogsNotReady(err error) bool {
	var apiErr *rdeapi.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}
