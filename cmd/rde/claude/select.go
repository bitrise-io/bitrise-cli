package claude

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil/picker"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
	"github.com/bitrise-io/bitrise-cli/internal/rde/localsession"
)

// errSelectionCancelled signals the user backed out of the image / machine-type
// picker (empty-with-no-default impossible, "q", EOF, or Ctrl-C). The command
// treats it as a clean exit, like the resume picker's cancel.
var errSelectionCancelled = errors.New("selection cancelled")

// selectImageAndMachineType resolves the image and machine type for a fresh
// session, mirroring the RDE web UI: pick an image first, then a machine type
// compatible with that image. When a still-valid combo is remembered for this
// project (and neither --image nor --machine-type is set), it first offers a
// one-step "use your last setup" menu so returning users don't re-pick the same
// pair. For each pick the choice is, in order: an explicit flag, the only option
// when there's just one (so a single machine type starts the session without a
// prompt), or the interactive picker (default = the per-project saved choice,
// else the backend default, else the first). When stdin/stderr isn't a terminal
// the default is used without prompting.
//
// The returned values are image and machine type NAMES, ready for CreateSession.
func selectImageAndMachineType(ctx context.Context, cmd *cobra.Command, svc *internalrde.Service, log *stepLogger, workspaceID, repoPath, flagImage, flagMachineType string) (string, string, error) {
	// Best-effort: a missing/corrupt prefs file yields the zero value, i.e.
	// "no prior choice", so we fall through to the backend default / first item.
	prefs, _ := localsession.LoadPrefs(repoPath)

	images, err := svc.ListImages(ctx, workspaceID)
	if err != nil {
		return "", "", fmt.Errorf("list images: %w", err)
	}
	if len(images) == 0 {
		return "", "", fmt.Errorf("no images are available in this workspace")
	}
	imageNames, backendDefaultImage := uniqueImageNames(images)

	// Fast path: reuse the remembered combo for this project in one step.
	if flagImage == "" && flagMachineType == "" && interactivePicker(cmd) && prefs.Image != "" && prefs.MachineType != "" {
		image, machineType, done, err := offerReuse(ctx, cmd, svc, log, workspaceID, imageNames, prefs)
		if err != nil || done {
			return image, machineType, err
		}
		// The remembered combo is stale, or the user chose "Change image" — fall
		// through to the full pick below (saved values still seed the defaults).
	}

	image, err := chooseOne(ctx, cmd, log, "image", "Select an image", imageNames, prefs.Image, backendDefaultImage, flagImage, nil)
	if err != nil {
		return "", "", err
	}

	machineType, err := chooseMachineForImage(ctx, cmd, svc, log, workspaceID, image, prefs.MachineType, flagMachineType)
	if err != nil {
		return "", "", err
	}
	return image, machineType, nil
}

// chooseMachineForImage resolves the machine type for the chosen image: it
// fetches the compatible types and defers to chooseOne, which auto-selects when
// only one is available — so a single compatible machine type starts the session
// without a machine-type prompt.
func chooseMachineForImage(ctx context.Context, cmd *cobra.Command, svc *internalrde.Service, log *stepLogger, workspaceID, image, prefMachineType, flagMachineType string) (string, error) {
	mts, err := svc.MachineTypesForImage(ctx, workspaceID, image)
	if err != nil {
		return "", err
	}
	if len(mts) == 0 {
		return "", fmt.Errorf("no machine types are compatible with image %q", image)
	}
	mtNames, backendDefaultMT := uniqueMachineTypeNames(mts)
	return chooseOne(ctx, cmd, log, "machine type", "Select a machine type", mtNames, prefMachineType, backendDefaultMT, flagMachineType, machineSpecHint)
}

// chooseOne resolves a single selection. An explicit flag is validated against
// the options and used as-is; a single option is auto-selected; a
// non-interactive stdin/stderr uses the resolved default without prompting;
// otherwise it shows the interactive picker. noun is used in messages
// ("image"); label heads the picker. descFn, when non-nil, derives an optional
// dim secondary hint for each row (used for machine-type specs). options must
// be non-empty.
func chooseOne(ctx context.Context, cmd *cobra.Command, log *stepLogger, noun, label string, options []string, prefName, backendDefault, flag string, descFn func(string) string) (string, error) {
	if flag != "" {
		if indexOf(options, flag) < 0 {
			return "", fmt.Errorf("%s %q is not available; choose one of: %s", noun, flag, strings.Join(options, ", "))
		}
		log.step("Using %s %q", noun, flag)
		return flag, nil
	}
	if len(options) == 1 {
		log.step("Using the only %s available: %s", noun, options[0])
		return options[0], nil
	}
	defaultIdx := resolveDefault(options, prefName, backendDefault)
	if !interactivePicker(cmd) {
		log.step("Using default %s (stdin is not a terminal): %s", noun, options[defaultIdx])
		return options[defaultIdx], nil
	}
	// Surface the default at the top of the list so it's the obvious
	// press-Enter choice, with the cursor and "(default)" badge on row 1.
	ordered := moveToFront(options, defaultIdx)
	items := make([]picker.Item, len(ordered))
	for i, opt := range ordered {
		items[i] = picker.Item{Title: opt}
		if descFn != nil {
			items[i].Desc = descFn(opt)
		}
	}
	idx, err := picker.Select(ctx, picker.Config{
		Prompt:     label,
		Items:      items,
		Cursor:     0,
		DefaultIdx: 0,
		In:         os.Stdin,
		Out:        cmd.ErrOrStderr(),
	})
	if errors.Is(err, picker.ErrCancelled) {
		return "", errSelectionCancelled
	}
	if err != nil {
		return "", err
	}
	return ordered[idx], nil
}

// reuseAction is which entry of the "use your last setup" menu was picked.
type reuseAction int

const (
	reuseUse reuseAction = iota
	reuseChangeImage
	reuseChangeMachine
)

// offerReuse shows the one-step "use your last setup" menu for a project that
// has a remembered image+machine combo. done=true means the combo was resolved
// here (reused, or customized through the menu) and the returned values are
// final; done=false with a nil error means the remembered combo is stale or the
// user asked to change the image, so the caller should run the full pick. A
// non-nil error (e.g. the user cancelled) aborts selection.
func offerReuse(ctx context.Context, cmd *cobra.Command, svc *internalrde.Service, log *stepLogger, workspaceID string, imageNames []string, prefs localsession.Prefs) (image, machineType string, done bool, err error) {
	// The remembered image must still be offered…
	if indexOf(imageNames, prefs.Image) < 0 {
		return "", "", false, nil
	}
	mts, err := svc.MachineTypesForImage(ctx, workspaceID, prefs.Image)
	if err != nil {
		return "", "", false, err
	}
	mtNames, _ := uniqueMachineTypeNames(mts)
	// …and the remembered machine type must still be compatible with it.
	if indexOf(mtNames, prefs.MachineType) < 0 {
		return "", "", false, nil
	}

	items, actions := buildReuseMenu(len(imageNames) > 1, len(mtNames) > 1, reuseSummary(prefs.Image, prefs.MachineType))
	// Nothing to customize (a single image and a single machine type): reuse
	// without prompting.
	if len(actions) == 1 {
		return prefs.Image, prefs.MachineType, true, nil
	}

	idx, err := picker.Select(ctx, picker.Config{
		Prompt:     "Last used for this project",
		Items:      items,
		Cursor:     0,
		DefaultIdx: 0,
		In:         os.Stdin,
		Out:        cmd.ErrOrStderr(),
	})
	if errors.Is(err, picker.ErrCancelled) {
		return "", "", false, errSelectionCancelled
	}
	if err != nil {
		return "", "", false, err
	}

	switch actions[idx] {
	case reuseChangeImage:
		// Caller runs the full image + machine pick.
		return "", "", false, nil
	case reuseChangeMachine:
		mt, err := chooseOne(ctx, cmd, log, "machine type", "Select a machine type", mtNames, prefs.MachineType, "", "", machineSpecHint)
		if err != nil {
			return "", "", false, err
		}
		return prefs.Image, mt, true, nil
	default: // reuseUse
		return prefs.Image, prefs.MachineType, true, nil
	}
}

// buildReuseMenu assembles the rows and matching actions for offerReuse. The
// "Change image" row appears only when more than one image exists, and "Change
// machine type" only when the remembered image has more than one compatible
// type — so we never offer a change with nothing to change.
func buildReuseMenu(multiImage, multiMachine bool, summary string) ([]picker.Item, []reuseAction) {
	items := []picker.Item{{Title: "Use this setup", Desc: summary}}
	actions := []reuseAction{reuseUse}
	if multiImage {
		items = append(items, picker.Item{Title: "Change image"})
		actions = append(actions, reuseChangeImage)
	}
	if multiMachine {
		items = append(items, picker.Item{Title: "Change machine type"})
		actions = append(actions, reuseChangeMachine)
	}
	return items, actions
}

// reuseSummary is the one-line "image · machine (specs)" shown on the reuse row.
func reuseSummary(image, machineType string) string {
	s := fmt.Sprintf("%s · %s", image, machineType)
	if hint := machineSpecHint(machineType); hint != "" {
		s += " · " + hint
	}
	return s
}

// interactivePicker reports whether an interactive picker can run: it reads keys
// from stdin and draws to stderr, so both must be a terminal.
func interactivePicker(cmd *cobra.Command) bool {
	return cmdutil.IsTerminal(os.Stdin) && cmdutil.WriterIsTTY(cmd.ErrOrStderr())
}

// machineSpecRe matches the "<vCPU>c-<RAM>g" tail of a machine-type name.
var machineSpecRe = regexp.MustCompile(`^(\d+)c-(\d+)g$`)

// machineSpecHint derives a "<n> vCPU · <m> GB" hint from a machine-type name
// by parsing its last '.'-separated segment (e.g. "g2.mac.m2pro.4c-6g" →
// "4c-6g" → "4 vCPU · 6 GB"). Returns "" when the segment doesn't match the
// "<n>c-<m>g" shape (e.g. "g2.mac"), so an unrecognized name simply shows no
// hint rather than a wrong one.
func machineSpecHint(name string) string {
	seg := name
	if i := strings.LastIndex(name, "."); i >= 0 {
		seg = name[i+1:]
	}
	m := machineSpecRe.FindStringSubmatch(seg)
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%s vCPU · %s GB", m[1], m[2])
}

// moveToFront returns options with the element at idx moved to the front,
// preserving the relative order of the rest. idx must be a valid index; idx 0
// (or out of range) returns options unchanged.
func moveToFront(options []string, idx int) []string {
	if idx <= 0 || idx >= len(options) {
		return options
	}
	out := make([]string, 0, len(options))
	out = append(out, options[idx])
	out = append(out, options[:idx]...)
	out = append(out, options[idx+1:]...)
	return out
}

// resolveDefault returns the index in names to preselect, applying the
// precedence saved-pref → backend default → first. A pref or backend default
// absent from names is skipped, so the result is always a valid index (names
// must be non-empty). This is also what makes a saved/backend-default machine
// type that isn't compatible with the chosen image fall back to first-available.
func resolveDefault(names []string, prefName, backendDefaultName string) int {
	if i := indexOf(names, prefName); i >= 0 {
		return i
	}
	if i := indexOf(names, backendDefaultName); i >= 0 {
		return i
	}
	return 0
}

// indexOf returns the index of target in names, or -1. An empty target never
// matches (it's the "unset" sentinel for prefs / backend defaults).
func indexOf(names []string, target string) int {
	if target == "" {
		return -1
	}
	for i, n := range names {
		if n == target {
			return i
		}
	}
	return -1
}

// uniqueImageNames returns the image names in catalog order with duplicates
// removed (a name can be offered by several clusters), plus the first name the
// backend flagged as default ("" if none).
func uniqueImageNames(images []internalrde.Image) (names []string, backendDefault string) {
	seen := make(map[string]bool, len(images))
	for _, im := range images {
		if !seen[im.Name] {
			seen[im.Name] = true
			names = append(names, im.Name)
		}
		if im.IsDefault && backendDefault == "" {
			backendDefault = im.Name
		}
	}
	return names, backendDefault
}

// uniqueMachineTypeNames is uniqueImageNames for machine types.
func uniqueMachineTypeNames(mts []internalrde.MachineType) (names []string, backendDefault string) {
	seen := make(map[string]bool, len(mts))
	for _, mt := range mts {
		if !seen[mt.Name] {
			seen[mt.Name] = true
			names = append(names, mt.Name)
		}
		if mt.IsDefault && backendDefault == "" {
			backendDefault = mt.Name
		}
	}
	return names, backendDefault
}
