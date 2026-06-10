package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalapp "github.com/bitrise-io/bitrise-cli/internal/app"
	"github.com/bitrise-io/bitrise-cli/internal/config"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

func newCreateCmd() *cobra.Command {
	var (
		repoURL     string
		branch      string
		title       string
		provider    string
		orgSlug     string
		stackID     string
		projectType string
		public      bool
		bitriseYML  string
	)

	c := &cobra.Command{
		Use:   "create",
		Short: "Register a new app on Bitrise",
		Long: `Register a new app (project) on Bitrise.

Auto-detection from the current git repo:
  --repo-url     git remote get-url origin
  --branch       git symbolic-ref --short HEAD (else "master")
  --title        last path segment of the repo URL (".git" stripped)

Workspace:
  --workspace is required if you belong to multiple workspaces.
  Otherwise it falls back to:
    1. default_workspace_id from config
    2. auto-detect when your account has exactly one workspace

bitrise.yml handling:
  --bitrise-yml PATH   upload that file as the app's config
  (no flag, ./bitrise.yml exists)   upload it
  (no flag, no file)   skip — server preset for --project-type takes effect

The new app's ID is saved as the global default app_id, so subsequent
commands (build trigger, build list, ...) target it without --app.`,
		Example: `  bitrise-cli app create
  bitrise-cli app create --repo-url https://github.com/me/proj --workspace acme
  bitrise-cli app create --bitrise-yml ./ci/bitrise.yml --stack osx-xcode-16.0.x
  bitrise-cli app create --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			format := cmdutil.ResolveFormat(cmd)

			// Resolve repo URL / branch / title from flags, falling back to
			// `git` invocations in the cwd. Any failure here is fatal — we
			// don't want to silently register the wrong repo.
			detector := internalapp.ExecGitDetector{}
			resolvedRepoURL, gitURLDetected, err := resolveRepoURL(ctx, repoURL, detector)
			if err != nil {
				return err
			}
			resolvedBranch, gitBranchDetected, err := resolveBranch(ctx, branch, detector)
			if err != nil {
				return err
			}

			// Resolve org from flag → config (auto-detect happens inside
			// the service when both are empty).
			resolvedOrg := orgSlug
			orgFromConfig := false
			if resolvedOrg == "" {
				resolvedOrg = config.FromContext(ctx).OrgSlug
				orgFromConfig = resolvedOrg != ""
			}

			// Resolve bitrise.yml: explicit path > ./bitrise.yml > none.
			yamlContent, yamlSource, err := resolveBitriseYML(bitriseYML)
			if err != nil {
				return err
			}

			// Friendly stderr breadcrumbs (silenced by -q).
			if !cmdutil.IsQuiet(cmd) {
				ew := cmdutil.NewErrWriter(cmd.ErrOrStderr())
				if gitURLDetected {
					ew.F("Detected repo URL from git: %s\n", resolvedRepoURL)
				}
				if gitBranchDetected {
					ew.F("Detected branch from git: %s\n", resolvedBranch)
				}
				if orgFromConfig {
					ew.F("Using workspace from config: %s\n", resolvedOrg)
				}
				switch yamlSource {
				case yamlSourceFlag:
					ew.F("Uploading bitrise.yml from %s\n", bitriseYML)
				case yamlSourceCwd:
					ew.F("Uploading bitrise.yml from ./bitrise.yml\n")
				case yamlSourceNone:
					ew.F("No bitrise.yml provided — using server preset for project-type=%s\n", defaultIfEmpty(projectType, internalapp.DefaultProjectType))
				}
				if ew.Err != nil {
					return ew.Err
				}
			}

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalapp.NewService(client)

			res, err := svc.Create(ctx, internalapp.CreateOptions{
				RepoURL:     resolvedRepoURL,
				Branch:      resolvedBranch,
				Title:       title,
				Provider:    provider,
				OrgSlug:     resolvedOrg,
				StackID:     stackID,
				ProjectType: projectType,
				Public:      public,
				BitriseYML:  yamlContent,
			})
			if err != nil {
				return err
			}

			// Persist the new slug as the global default. Per-directory
			// .bitrise-cli.yml still wins at runtime — warn if so.
			if err := persistAppSlug(cmd, res.Slug); err != nil {
				return err
			}

			return output.Render(cmd.OutOrStdout(), format, res, renderCreateText)
		},
	}

	c.Flags().StringVar(&repoURL, "repo-url", "", "git repo URL (default: 'git remote get-url origin' in cwd)")
	c.Flags().StringVar(&branch, "branch", "", "default branch (default: 'git symbolic-ref --short HEAD', else 'master')")
	c.Flags().StringVar(&title, "title", "", "app title (default: last path segment of repo URL)")
	c.Flags().StringVar(&provider, "provider", "auto", "git provider: auto, github, gitlab, bitbucket, custom")
	c.Flags().StringVar(&orgSlug, "workspace", "", "workspace ID to own the app")
	c.Flags().StringVar(&stackID, "stack", "", fmt.Sprintf("build stack ID (default %q)", internalapp.DefaultStackID))
	c.Flags().StringVar(&projectType, "project-type", "", fmt.Sprintf("project type for server-side preset (default %q)", internalapp.DefaultProjectType))
	c.Flags().BoolVar(&public, "public", false, "create as a public app")
	c.Flags().StringVar(&bitriseYML, "bitrise-yml", "", "path to bitrise.yml to upload (default: ./bitrise.yml if present, else skip)")

	_ = c.RegisterFlagCompletionFunc("provider", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"auto", "github", "gitlab", "bitbucket", "custom"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = c.RegisterFlagCompletionFunc("project-type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"ios", "android", "flutter", "react-native", "xamarin", "cordova", "ionic", "other"}, cobra.ShellCompDirectiveNoFileComp
	})

	return c
}

// resolveRepoURL returns the explicit flag value or the cwd's git origin.
// The second return is true when the value came from git (used to decide
// whether to log "Detected repo URL from git" on stderr).
func resolveRepoURL(ctx context.Context, flagValue string, d internalapp.GitDetector) (string, bool, error) {
	if flagValue != "" {
		return flagValue, false, nil
	}
	v, err := d.RemoteURL(ctx)
	if err != nil {
		return "", false, fmt.Errorf("detect git remote: %w", err)
	}
	if v == "" {
		return "", false, errors.New("--repo-url is required (no git origin detected in this directory)")
	}
	return v, true, nil
}

func resolveBranch(ctx context.Context, flagValue string, d internalapp.GitDetector) (string, bool, error) {
	if flagValue != "" {
		return flagValue, false, nil
	}
	v, err := d.CurrentBranch(ctx)
	if err != nil {
		return "", false, fmt.Errorf("detect git branch: %w", err)
	}
	return v, v != "", nil // empty → service falls back to DefaultBranchFallback
}

type yamlSource int

const (
	yamlSourceNone yamlSource = iota
	yamlSourceFlag
	yamlSourceCwd
)

func resolveBitriseYML(flagPath string) (string, yamlSource, error) {
	if flagPath != "" {
		data, err := os.ReadFile(flagPath) //nolint:gosec // user-supplied path is intentional
		if err != nil {
			return "", yamlSourceNone, fmt.Errorf("read %s: %w", flagPath, err)
		}
		return string(data), yamlSourceFlag, nil
	}
	const cwdPath = "bitrise.yml"
	if _, err := os.Stat(cwdPath); err == nil {
		data, err := os.ReadFile(cwdPath)
		if err != nil {
			return "", yamlSourceNone, fmt.Errorf("read %s: %w", cwdPath, err)
		}
		return string(data), yamlSourceCwd, nil
	}
	return "", yamlSourceNone, nil
}

// persistAppSlug saves slug as the global default app_id, mirroring
// `bitrise-cli config set app_id <id>`. Logs the result to stderr
// (suppressed by -q). Warns if a per-directory .bitrise-cli.yml will
// continue to override the global default at runtime.
func persistAppSlug(cmd *cobra.Command, slug string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Set(config.KeyAppSlug, slug); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if cmdutil.IsQuiet(cmd) {
		return nil
	}
	cfgPath, _ := config.Path()
	ew := cmdutil.NewErrWriter(cmd.ErrOrStderr())
	ew.F("Set %s=%s in %s\n", config.KeyAppSlug, slug, cfgPath)

	// Per-directory file overrides the global default — surface the
	// conflict so the user isn't surprised when their next command picks
	// a different app.
	dirCfg, dirPath, derr := config.LoadDir()
	if derr == nil && dirPath != "" && dirCfg.AppSlug != "" && dirCfg.AppSlug != slug {
		ew.F("Note: %s pins %s=%s, which still wins at runtime\n", dirPath, config.KeyAppSlug, dirCfg.AppSlug)
	}
	return ew.Err
}

func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func renderCreateText(w io.Writer, r internalapp.CreateResult) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-22s", label))
	}
	ew.F("%s %s\n", s.Success.Render("✓"), s.Success.Render(fmt.Sprintf("Created app %s", r.Slug)))
	ew.F("%s%s\n", lbl("  Title:"), r.Title)
	ew.F("%s%s\n", lbl("  ID:"), s.Slug.Render(r.Slug))
	ew.F("%s%s\n", lbl("  Repo URL:"), s.URL.Render(r.RepoURL))
	ew.F("%s%s\n", lbl("  Default branch:"), r.DefaultBranch)
	ew.F("%s%s\n", lbl("  Workspace:"), r.OrgSlug)
	ew.F("%s%s\n", lbl("  Stack:"), r.StackID)
	ew.F("%s%s\n", lbl("  Project type:"), r.ProjectType)
	ew.F("%s%s\n", lbl("  Build trigger token:"), s.Slug.Render(r.BuildTriggerToken))
	if !r.BitriseYMLUploaded {
		ew.F("%s%s\n", lbl("  Config:"), s.Dim.Render("server preset"))
	}
	return ew.Err
}
