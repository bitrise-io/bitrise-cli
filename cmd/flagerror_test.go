package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRewriteFlagError(t *testing.T) {
	root := &cobra.Command{Use: "bitrise-cli", Version: "test"}
	root.PersistentFlags().String("output", "", "")
	build := &cobra.Command{Use: "build"}
	build.Flags().String("app", "", "")
	root.AddCommand(build)

	cases := []struct {
		name    string
		cmd     *cobra.Command
		err     error
		args    []string
		wantSub string
	}{
		{"nil", root, nil, []string{"-help"}, ""},
		{"help on root", root, errors.New("unknown shorthand flag: 'e' in -elp"), []string{"-help"}, "did you mean --help?"},
		{"version on root", root, errors.New("unknown shorthand flag: 'e' in -ersion"), []string{"-version"}, "did you mean --version?"},
		{"output via parent walk", build, errors.New("unknown shorthand flag: 'u' in -utput"), []string{"build", "-output", "json"}, "did you mean --output?"},
		{"version via parent (lazy init)", build, errors.New("unknown shorthand flag: 'v' in -version"), []string{"build", "-version"}, "did you mean --version?"},
		{"unknown long flag", root, errors.New("unknown flag: --foo"), []string{"--foo"}, ""},
		{"unregistered word", root, errors.New("unknown shorthand flag: 'y' in -yz"), []string{"-xyz"}, ""},
		{"single letter dash", root, errors.New("unknown shorthand flag: 'z' in -z"), []string{"-z"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteFlagError(tc.cmd, tc.err, tc.args)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("nil err: got %v", got)
				}
				return
			}
			if !errors.Is(got, tc.err) {
				t.Fatalf("wrapped error must satisfy errors.Is(got, original); got=%q", got)
			}
			if tc.wantSub == "" {
				if got.Error() != tc.err.Error() {
					t.Fatalf("expected unchanged %q, got %q", tc.err, got)
				}
				return
			}
			if !strings.Contains(got.Error(), tc.wantSub) {
				t.Fatalf("expected %q to contain %q", got.Error(), tc.wantSub)
			}
		})
	}
}
