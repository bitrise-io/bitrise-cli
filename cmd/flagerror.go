package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
)

var unknownShorthandRe = regexp.MustCompile(`^unknown shorthand flag: '.' in -[A-Za-z0-9]+$`)

// rewriteFlagError appends a "did you mean --foo?" hint when the user typed a
// single-dash long flag like `-help` and the resulting cluster failed to
// parse. pflag's error reports only the unparsed remainder of the cluster
// (`-help` errors as `'e' in -elp` once `h` is consumed), so we recover the
// original token by scanning args for a `-<word>` whose `<word>` matches a
// long flag registered on cmd or any of its parents.
func rewriteFlagError(cmd *cobra.Command, err error, args []string) error {
	if err == nil {
		return nil
	}
	if !unknownShorthandRe.MatchString(err.Error()) {
		return err
	}
	for _, a := range args {
		if len(a) < 3 || a[0] != '-' || a[1] == '-' {
			continue
		}
		word := a[1:]
		for c := cmd; c != nil; c = c.Parent() {
			c.InitDefaultHelpFlag()
			c.InitDefaultVersionFlag()
			if c.Flags().Lookup(word) != nil || c.PersistentFlags().Lookup(word) != nil {
				return fmt.Errorf("%w (did you mean --%s?)", err, word)
			}
		}
	}
	return err
}

func flagErrorFunc(cmd *cobra.Command, err error) error {
	return rewriteFlagError(cmd, err, os.Args[1:])
}
