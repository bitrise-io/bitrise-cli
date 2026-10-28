package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// parseLabelFlags converts repeatable key=value flag values into a label
// map; flagName names the flag in error messages. Only the shape is checked
// here — key and value charset/length, entry count, and the reserved
// "bitrise.io/" key prefix are enforced by the backend, whose field
// violations surface through the API error.
func parseLabelFlags(flagName string, kvs []string) (map[string]string, error) {
	if len(kvs) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("%s %q: expected key=value", flagName, kv)
		}
		out[k] = v
	}
	return out, nil
}

// validateLabelSelectors rejects selector shapes that can never match (bare
// keys, empty keys or values — label values are non-empty by contract). The
// strings otherwise pass to the backend verbatim, which enforces the full
// rules (at most 8 selectors, no duplicate keys).
func validateLabelSelectors(selectors []string) error {
	for _, sel := range selectors {
		k, v, ok := strings.Cut(sel, "=")
		if !ok || k == "" || v == "" {
			return fmt.Errorf("--label-selector %q: expected key=value", sel)
		}
	}
	return nil
}

// formatTime is the canonical timestamp format for session human output.
// Empty pointer renders as "" so renderers can shove it directly into a
// table cell without conditionals.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// statusStyle picks the right semantic color for a session status string.
// The status comes from internal/rde — already lowercased, prefix-stripped.
func statusStyle(s style.Styles, status string) lipgloss.Style {
	switch status {
	case "running":
		return s.BuildStatus("success")
	case "pending", "starting", "terminating", "draining":
		return s.BuildStatus("in-progress")
	case "terminated", "drained":
		return s.Dim
	case "failed":
		return s.BuildStatus("failed")
	}
	return s.Dim
}

// isTerminalStatus reports whether a session status (lowercased,
// prefix-stripped, as internal/rde delivers it) means the VM is gone, so
// connection details must not be shown.
func isTerminalStatus(status string) bool {
	switch status {
	case "terminated", "stopped", "failed", "drained":
		return true
	}
	return false
}

// diskStatusText renders a terminated session's persistent-disk status with
// a hint about whether the session can still be restored. Callers guard on a
// non-empty status, so the default branch only fires for a value added to
// the backend after this code was written.
func diskStatusText(s style.Styles, status string) string {
	switch status {
	case internalrde.DiskStatusAvailable:
		return s.Success.Render("available") + s.Dim.Render(" — restorable")
	case internalrde.DiskStatusUnavailableSoon:
		return s.Warn.Render("expiring soon") + s.Dim.Render(" — restore within ~1 week")
	case internalrde.DiskStatusUnavailable:
		return s.Failure.Render("unavailable") + s.Dim.Render(" — cannot be restored")
	}
	return status
}

// warmStateText renders a session's relation to its warm pool with the one
// fact a caller wants from it: whether the session skipped the VM boot.
// Unknown states (a value added after this code was written) render as-is.
func warmStateText(s style.Styles, state string) string {
	switch state {
	case internalrde.WarmStateClaimed:
		return s.Success.Render(state) + s.Dim.Render(" — handed out pre-booted from the warm pool")
	case internalrde.WarmStateCold:
		return s.Warn.Render(state) + s.Dim.Render(" — no warm session was available; created from the pool's configuration")
	case "warming", "ready":
		return state + s.Dim.Render(" — part of the pool's inventory, not yet claimed")
	}
	return state
}

// deviceStateStyle colors the device readiness like statusStyle colors the
// session status: ready is good, failed is bad, anything else is in flight.
func deviceStateStyle(s style.Styles, state string) lipgloss.Style {
	switch state {
	case "ready":
		return s.Success
	case "failed":
		return s.Failure
	default:
		return s.Dim
	}
}

// lifecyclePhaseText renders a session's lifecycle phase (internal/rde
// delivers it lowercased, prefix-stripped) with the one fact a caller wants
// from it: whether the setup is still running, done, or broken. Unknown
// phases (a value added after this code was written) render as-is.
func lifecyclePhaseText(s style.Styles, phase string) string {
	words := strings.ReplaceAll(phase, "_", " ")
	switch phase {
	case "ready":
		return s.Success.Render(words) + s.Dim.Render(" — every lifecycle hook is done")
	case "setting_up":
		return s.BuildStatus("in-progress").Render(words) + s.Dim.Render(" — On create is running; exec, ssh, upload, download and the terminal open when it finishes")
	case "personalizing":
		return s.BuildStatus("in-progress").Render(words) + s.Dim.Render(" — the system setup / On claim is running")
	case "restoring":
		return s.BuildStatus("in-progress").Render(words) + s.Dim.Render(" — On restore is running")
	case "setup_failed", "personalization_failed", "restore_failed":
		return s.Failure.Render(words) + s.Dim.Render(" — the last hook failed; see the lifecycle hooks below and 'session logs'")
	case "warming", "ready_to_claim":
		return words + s.Dim.Render(" — warm pool inventory, not yet claimed")
	}
	return words
}

// hookLabel is the human name of a lifecycle hook ("on_create" → "On
// create"); the platform's own "personalize" step is the system setup (its injected scripts).
func hookLabel(hook string) string {
	switch hook {
	case "on_create":
		return "On create"
	case "on_claim":
		return "On claim"
	case "on_restore":
		return "On restore"
	case "personalize":
		return "System setup"
	}
	return hook
}

// hookRunText renders one hook run's state with what matters about it: the
// exit code of a failure, and whether that failure was RDE ending an On
// create that did not exit within its limit. Unknown states render as-is.
func hookRunText(s style.Styles, r internalrde.HookRun) string {
	switch r.State {
	case "succeeded":
		return s.Success.Render(r.State)
	case "failed":
		if r.Hook == internalrde.HookOnCreate && r.ExitCode == internalrde.HookExitCodeOnCreateLimit {
			return s.Failure.Render(fmt.Sprintf("failed (exit %d)", r.ExitCode)) + s.Dim.Render(" — On create did not finish within 2 hours; long-running work must be started in the background so the script can exit")
		}
		return s.Failure.Render(fmt.Sprintf("failed (exit %d)", r.ExitCode))
	case "interrupted":
		return s.Failure.Render(r.State) + s.Dim.Render(" — the machine stopped mid-run")
	case "skipped":
		return s.Dim.Render("skipped — an earlier hook failed")
	case "pending", "running":
		return s.BuildStatus("in-progress").Render(r.State)
	}
	return r.State
}
