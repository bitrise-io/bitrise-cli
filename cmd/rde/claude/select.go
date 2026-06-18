package claude

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
	"github.com/bitrise-io/bitrise-cli/internal/rde/localsession"
)

// errSelectionCancelled signals the user backed out of the image / machine-type
// picker (empty-with-no-default impossible, "q", EOF, or Ctrl-C). The command
// treats it as a clean exit, like the resume picker's cancel.
var errSelectionCancelled = errors.New("selection cancelled")

// selectImageAndMachineType resolves the image and machine type for a fresh
// session, mirroring the RDE web UI: pick an image first, then a machine type
// compatible with that image. For each step the choice is, in order: an
// explicit --image/--machine-type flag, the only option when there's just one,
// or an interactive numbered picker (the default preselected from the per-repo
// saved choice, else the backend-marked default, else the first item). When
// stdin isn't a terminal the default is used without prompting.
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

	image, err := chooseOne(ctx, cmd, log, "image", "Select an image", imageNames, prefs.Image, backendDefaultImage, flagImage)
	if err != nil {
		return "", "", err
	}

	mts, err := svc.MachineTypesForImage(ctx, workspaceID, image)
	if err != nil {
		return "", "", err
	}
	if len(mts) == 0 {
		return "", "", fmt.Errorf("no machine types are compatible with image %q", image)
	}
	mtNames, backendDefaultMT := uniqueMachineTypeNames(mts)

	machineType, err := chooseOne(ctx, cmd, log, "machine type", "Select a machine type", mtNames, prefs.MachineType, backendDefaultMT, flagMachineType)
	if err != nil {
		return "", "", err
	}
	return image, machineType, nil
}

// chooseOne resolves a single selection. An explicit flag is validated against
// the options and used as-is; a single option is auto-selected; a non-terminal
// stdin uses the resolved default without prompting; otherwise it shows an
// interactive numbered picker. noun is used in messages ("image"); label heads
// the picker. options must be non-empty.
func chooseOne(ctx context.Context, cmd *cobra.Command, log *stepLogger, noun, label string, options []string, prefName, backendDefault, flag string) (string, error) {
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
	if !cmdutil.IsTerminal(os.Stdin) {
		log.step("Using default %s (stdin is not a terminal): %s", noun, options[defaultIdx])
		return options[defaultIdx], nil
	}
	// Surface the default at the top of the list so it's easy to recognize as
	// the press-Enter choice, then prompt with it preselected at position 1.
	ordered := moveToFront(options, defaultIdx)
	idx, err := promptChoice(ctx, cmd, label, ordered, 0)
	if err != nil {
		return "", err
	}
	return ordered[idx], nil
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

// promptChoice renders a numbered list of options to stderr and reads a
// selection from stdin. Enter accepts the default (defaultIdx); a number 1-N
// picks that option; "q", EOF (Ctrl-D), or Ctrl-C (which cancels ctx) back out
// with errSelectionCancelled. Returns the chosen index. Mirrors the resume
// picker (resume.go pickRecord).
func promptChoice(ctx context.Context, cmd *cobra.Command, label string, options []string, defaultIdx int) (int, error) {
	ew := cmdutil.NewErrWriter(cmd.ErrOrStderr())
	ew.F("%s:\n", label)
	for i, opt := range options {
		marker := ""
		if i == defaultIdx {
			marker = "  (default)"
		}
		ew.F("  %d) %s%s\n", i+1, opt, marker)
	}
	ew.F("Choice (1-%d) [%d], or q to cancel: ", len(options), defaultIdx+1)
	if ew.Err != nil {
		return 0, ew.Err
	}

	line, err := readLine(ctx, os.Stdin)
	if err != nil {
		// Ctrl-C (ctx cancelled) or Ctrl-D (EOF) → treat as cancel.
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			return 0, errSelectionCancelled
		}
		return 0, err
	}
	line = strings.TrimSpace(line)
	switch strings.ToLower(line) {
	case "q", "quit", "exit":
		return 0, errSelectionCancelled
	case "":
		return defaultIdx, nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(options) {
		return 0, fmt.Errorf("invalid selection %q", line)
	}
	return n - 1, nil
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
