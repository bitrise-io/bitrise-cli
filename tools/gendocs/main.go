// Command gendocs renders the bitrise-cli command tree into a markdown
// reference, one file per command, under docs/cli.
//
// Run it via `make docs`. The output is committed, so every change to a
// command, flag, or help text shows up as a reviewable diff in the PR that
// makes it; `make docs-check` (run in CI) fails when the committed files
// drift from the command definitions.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"

	"github.com/bitrise-io/bitrise-cli/cmd"
)

const outDir = "docs/cli"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func run() error {
	root := cmd.Root()
	// The default footer stamps the generation date into every file, which
	// would dirty the tree on each regeneration; git history already records
	// when the docs changed.
	root.DisableAutoGenTag = true

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}
	// Remove previously generated files so commands deleted from the tree
	// don't leave stale pages behind.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "bitrise-cli") && strings.HasSuffix(e.Name(), ".md") {
			if err := os.Remove(filepath.Join(outDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return doc.GenMarkdownTree(root, outDir)
}
