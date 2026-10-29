package rde

import (
	"context"
	"fmt"
	"strings"
	"time"

	rdeapi "github.com/bitrise-io/bitrise-cli/bitriseapi/rde"
)

// Friendly --stage values accepted by the CLI. The backend takes the numeric
// LogStage enum; these map to it in logStageToAPI. The backend calls the
// second stage "main" internally; the user-facing name is "startup" (it runs
// the session's startup script — on a session that runs lifecycle hooks it is
// the combined log of the platform's startup stage and every hook).
//
// The hook stages exist only on sessions that run lifecycle hooks (a template
// with hooks, or a legacy template mapped at creation): each one is that
// hook's output, every run in order; "system" is the platform's own output
// (the warmup stage, then the startup stage without the hooks' lines, plus the
// system setup step).
const (
	LogStageWarmup    = "warmup"     // runs once at session creation
	LogStageStartup   = "startup"    // runs on every session start/restart
	LogStageOnCreate  = "on-create"  // the On create hook (lifecycle hooks only)
	LogStageOnClaim   = "on-claim"   // the On claim hook (lifecycle hooks only)
	LogStageOnRestore = "on-restore" // the On restore hook (lifecycle hooks only)
	LogStageSystem    = "system"     // the platform's own output (lifecycle hooks only)
	// LogStagePersonalize is the platform's "System setup" step alone (the
	// "personalize" hook run), without the rest of the system output.
	LogStagePersonalize = "personalize"
)

// LogStages is the ordered set of valid --stage values, for validation and
// shell completion.
var LogStages = []string{LogStageWarmup, LogStageStartup, LogStageOnCreate, LogStageOnClaim, LogStageOnRestore, LogStageSystem, LogStagePersonalize}

// ValidLogStage reports whether stage is one of LogStages.
func ValidLogStage(stage string) bool {
	for _, s := range LogStages {
		if s == stage {
			return true
		}
	}
	return false
}

// LogStageList renders LogStages for error messages and help text
// ("warmup, startup, on-create, on-claim, on-restore, system or personalize").
func LogStageList() string {
	return strings.Join(LogStages[:len(LogStages)-1], ", ") + " or " + LogStages[len(LogStages)-1]
}

// logStageToAPI maps a friendly stage name to the numeric LogStage enum string
// the backend expects in the path (1=warmup, 2=main/startup, 3=on_create,
// 4=on_claim, 5=on_restore, 6=system, 7=personalize).
func logStageToAPI(stage string) (string, error) {
	switch stage {
	case LogStageWarmup:
		return "1", nil
	case LogStageStartup:
		return "2", nil
	case LogStageOnCreate:
		return "3", nil
	case LogStageOnClaim:
		return "4", nil
	case LogStageOnRestore:
		return "5", nil
	case LogStageSystem:
		return "6", nil
	case LogStagePersonalize:
		return "7", nil
	default:
		return "", fmt.Errorf("invalid stage %q (must be %s)", stage, LogStageList())
	}
}

// StreamSessionLogs streams one stage's log for a session, invoking fn for each
// content chunk's text in order. stage is a friendly name (warmup/startup).
//
// idleTimeout controls when to stop: 0 follows live until ctx is cancelled
// (Ctrl-C); a positive value returns once no new content has arrived for that
// long, which delivers the replayed log-so-far and then exits.
//
// A pre-stream "logs not ready" condition surfaces as a 404 *rdeapi.APIError,
// which the cmd layer distinguishes to decide between a friendly exit and a
// follow-mode retry.
func (s *Service) StreamSessionLogs(ctx context.Context, workspaceID, sessionID, stage string, idleTimeout time.Duration, fn func(string) error) error {
	return s.StreamSessionLogsAttempt(ctx, workspaceID, sessionID, stage, 0, idleTimeout, fn)
}

// HookLogStage reports whether a stage is one hook's log (where --attempt
// applies): on-create, on-claim, on-restore and personalize.
func HookLogStage(stage string) bool {
	switch stage {
	case LogStageOnCreate, LogStageOnClaim, LogStageOnRestore, LogStagePersonalize:
		return true
	}
	return false
}

// StreamSessionLogsAttempt is StreamSessionLogs narrowed to one attempt of a
// hook stage (1-based, as 'session view' numbers the runs); 0 streams every
// attempt in order. attempt is rejected on the non-hook stages.
func (s *Service) StreamSessionLogsAttempt(ctx context.Context, workspaceID, sessionID, stage string, attempt int, idleTimeout time.Duration, fn func(string) error) error {
	if s.client == nil {
		return errClient()
	}
	apiStage, err := logStageToAPI(stage)
	if err != nil {
		return err
	}
	if attempt < 0 {
		return fmt.Errorf("attempt must be 1 or higher (0 = every attempt)")
	}
	if attempt > 0 && !HookLogStage(stage) {
		return fmt.Errorf("--attempt applies to a hook's log only (on-create, on-claim, on-restore or personalize), not %q", stage)
	}
	return s.client.StreamSessionLogsAttempt(ctx, workspaceID, sessionID, apiStage, attempt, idleTimeout, func(chunk rdeapi.LogChunk) error {
		return fn(chunk.LogContent)
	})
}
