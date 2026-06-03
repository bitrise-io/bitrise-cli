package cmd

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// Build-info variables. Defaults are used when the binary is built with a
// plain `go build`; CI builds inject real values via -ldflags:
//
//	go build -ldflags "-X github.com/bitrise-io/bitrise-cli/cmd.version=1.2.3 \
//	                  -X github.com/bitrise-io/bitrise-cli/cmd.commit=$SHA"
//
// buildNumber is the CI build number that produced the binary (empty for
// dev builds). The release pipeline injects it from $BITRISE_BUILD_NUMBER so
// a published binary can be traced back to the exact build that made it.
var (
	version     = "dev"
	commit      = ""
	buildNumber = ""
)

// versionInfo is the JSON shape of `bitrise-cli version`.
type versionInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit,omitempty"`
	BuildNumber string `json:"build_number,omitempty"`
	BuildTime   string `json:"build_time,omitempty"`
	GoVersion   string `json:"go_version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
}

// readVersionInfo merges ldflag-injected values with what go embeds via
// debug.ReadBuildInfo (vcs.revision, vcs.time) when ldflags weren't used.
func readVersionInfo() versionInfo {
	v := versionInfo{
		Version:     version,
		Commit:      commit,
		BuildNumber: buildNumber,
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if v.Commit == "" {
					v.Commit = s.Value
				}
			case "vcs.time":
				v.BuildTime = s.Value
			}
		}
	}
	return v
}

// versionString is the value cobra renders after "bitrise-cli version "
// when --version is passed. Cobra adds the binary-name prefix; we only
// emit the version + short commit.
func versionString() string {
	v := readVersionInfo()
	if v.Commit != "" {
		short := v.Commit
		if len(short) > 7 {
			short = short[:7]
		}
		return fmt.Sprintf("%s (%s)", v.Version, short)
	}
	return v.Version
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build info",
		Long: `Print version, commit, and build info.

In JSON mode, all fields are emitted; missing values are omitted.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return output.Render(cmd.OutOrStdout(), resolveFormat(cmd), readVersionInfo(), renderVersionHuman)
		},
	}
}

func renderVersionHuman(w io.Writer, v versionInfo) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-16s", label))
	}
	ew.F("%s %s\n", s.Bold.Render("bitrise-cli"), v.Version)
	if v.Commit != "" {
		ew.F("%s%s\n", lbl("Commit:"), s.Slug.Render(v.Commit))
	}
	if v.BuildNumber != "" {
		ew.F("%s%s\n", lbl("Build:"), v.BuildNumber)
	}
	if v.BuildTime != "" {
		ew.F("%s%s\n", lbl("Built:"), v.BuildTime)
	}
	ew.F("%s%s\n", lbl("Go:"), v.GoVersion)
	ew.F("%s%s/%s\n", lbl("Platform:"), v.OS, v.Arch)
	return ew.Err
}
