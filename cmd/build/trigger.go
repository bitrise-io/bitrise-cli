package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	internalbuild "github.com/bitrise-io/bitrise-cli/internal/build"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

func newTriggerCmd() *cobra.Command {
	var (
		workflow      string
		pipeline      string
		branch        string
		branchDest    string
		tag           string
		commitHash    string
		commitMessage string
		envJSON       string
		priority      int
		pullRequestID int
		wait          bool
		watch         bool
		interval      time.Duration
	)

	c := &cobra.Command{
		Use:   "trigger",
		Short: "Start a new build",
		Long: `Start a new build on the given app.

Required flags:
  --app SLUG         (or BITRISE_APP_SLUG env var)

Optional flags:
  --workflow ID          workflow ID (mutually exclusive with --pipeline); Bitrise
                         selects the appropriate workflow from the trigger map if omitted
  --pipeline ID          pipeline ID (mutually exclusive with --workflow)
  --branch BRANCH        branch to build (default "main" for branch builds)
  --branch-dest BRANCH   target branch for pull-request builds
  --tag TAG              tag to build
  --commit-hash HASH     commit hash to build from
  --commit-message MSG   commit message to record
  --pull-request-id ID   pull request ID for PR builds
  --priority N           build priority (-1 = low, 0 = normal, 1 = high)
  --env JSON             environment variables as a JSON object, e.g. '{"KEY":"value"}'
  --wait                 wait for the build to finish without streaming logs; exits 0 on
                         success, 1 on failure. With --output json the final build record
                         is written to stdout.
  --watch                stream build logs until the build finishes; exits 0 on success,
                         1 on failure. With --output json logs go to stderr and the final
                         build record is written to stdout.
  --interval DURATION    polling interval when --wait or --watch is active (default 3s)`,
		Example: `  bitrise-cli build trigger --app my-app-slug --workflow primary
  bitrise-cli build trigger --app my-app-slug --workflow deploy --branch release/1.2 --output json
  bitrise-cli build trigger --app my-app-slug --pipeline my-pipeline --branch main
  bitrise-cli build trigger --app my-app-slug --workflow primary --tag v1.2.3
  bitrise-cli build trigger --app my-app-slug --workflow primary --env '{"MY_VAR":"hello","OTHER":"world"}'
  bitrise-cli build trigger --app my-app-slug --workflow primary --wait
  bitrise-cli build trigger --app my-app-slug --workflow primary --watch
  bitrise-cli build trigger --app my-app-slug --workflow primary --watch --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			appSlug, err := cmdutil.ResolveAppSlug(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)

			// Default branch to "main" for branch builds when no tag is specified.
			if branch == "" && tag == "" {
				branch = "main"
			}

			var envs []internalbuild.TriggerEnv
			if envJSON != "" {
				var raw map[string]string
				if err := json.Unmarshal([]byte(envJSON), &raw); err != nil {
					return fmt.Errorf("--env: invalid JSON object: %w", err)
				}
				for k, v := range raw {
					envs = append(envs, internalbuild.TriggerEnv{Key: k, Value: v})
				}
			}

			client, err := cmdutil.NewAPIClient(cmd)
			if err != nil {
				return err
			}
			svc := internalbuild.NewService(client)
			b, err := svc.Trigger(cmd.Context(), internalbuild.TriggerRequest{
				AppSlug:       appSlug,
				Workflow:      workflow,
				Pipeline:      pipeline,
				Branch:        branch,
				BranchDest:    branchDest,
				Tag:           tag,
				CommitHash:    commitHash,
				CommitMessage: commitMessage,
				PullRequestID: pullRequestID,
				Priority:      priority,
				Environments:  envs,
			})
			if err != nil {
				return err
			}

			if !wait && !watch {
				return output.Render(cmd.OutOrStdout(), format, b, renderTriggerHero)
			}

			if watch {
				// --watch: stream logs until the build finishes.
				// In JSON mode logs go to stderr so stdout carries only the final build JSON.
				logWriter := io.Writer(cmd.OutOrStdout())
				if format == output.JSON {
					logWriter = cmd.ErrOrStderr()
				}
				return runWatch(cmd, svc, b, interval, logWriter, format)
			}

			// --wait: silent block until the build finishes; no log output.
			if !cmdutil.IsQuiet(cmd) {
				headerEW := cmdutil.NewErrWriter(cmd.ErrOrStderr())
				headerEW.F("Waiting for build #%d to finish\n", b.BuildNumber)
				if b.BuildURL != "" {
					headerEW.F("→ %s\n", b.BuildURL)
				}
				if headerEW.Err != nil {
					return headerEW.Err
				}
			}
			finalBuild, err := svc.WaitForCompletion(cmd.Context(), b.AppSlug, b.Slug, interval)
			if errors.Is(err, context.Canceled) {
				detachEW := cmdutil.NewErrWriter(cmd.ErrOrStderr())
				detachEW.F("\nDetached — build is still running.\n")
				detachEW.F("Use 'bitrise-cli build watch %s' to resume.\n", b.Slug)
				return detachEW.Err
			}
			if err != nil {
				return err
			}
			if format == output.JSON {
				return output.Render(cmd.OutOrStdout(), format, finalBuild, renderBuildText)
			}
			if !cmdutil.IsQuiet(cmd) {
				footerEW := cmdutil.NewErrWriter(cmd.ErrOrStderr())
				footerEW.F("Build #%d finished: %s%s\n", finalBuild.BuildNumber, finalBuild.Status, buildElapsed(finalBuild))
				if footerEW.Err != nil {
					return footerEW.Err
				}
			}
			if finalBuild.Status != "success" && finalBuild.Status != "aborted-with-success" {
				cmdutil.SilenceRootErrors(cmd)
				return fmt.Errorf("build %s", finalBuild.Status)
			}
			return nil
		},
	}

	c.Flags().StringVar(&workflow, "workflow", "", "workflow ID to trigger (mutually exclusive with --pipeline)")
	c.Flags().StringVar(&pipeline, "pipeline", "", "pipeline ID to trigger (mutually exclusive with --workflow)")
	c.Flags().StringVar(&branch, "branch", "", `branch to build (default "main" for branch builds)`)
	c.Flags().StringVar(&branchDest, "branch-dest", "", "target branch for pull-request builds")
	c.Flags().StringVar(&tag, "tag", "", "tag to build")
	c.Flags().StringVar(&commitHash, "commit-hash", "", "commit hash to build")
	c.Flags().StringVar(&commitMessage, "commit-message", "", "commit message to record")
	c.Flags().StringVar(&envJSON, "env", "", `environment variables as a JSON object, e.g. '{"KEY":"value"}'`)
	c.Flags().IntVar(&priority, "priority", 0, "build priority (-1 = low, 0 = normal, 1 = high)")
	c.Flags().IntVar(&pullRequestID, "pull-request-id", 0, "pull request ID for PR builds")
	c.Flags().BoolVar(&wait, "wait", false, "block until the build finishes without streaming logs (exit code reflects build outcome)")
	c.Flags().BoolVar(&watch, "watch", false, "stream build logs until the build finishes (exit code reflects build outcome)")
	c.Flags().DurationVar(&interval, "interval", 3*time.Second, "polling interval when --wait or --watch is active")
	c.MarkFlagsMutuallyExclusive("workflow", "pipeline")
	c.MarkFlagsMutuallyExclusive("wait", "watch")

	_ = c.RegisterFlagCompletionFunc("priority", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"-1\tlow priority", "0\tnormal priority", "1\thigh priority"}, cobra.ShellCompDirectiveNoFileComp
	})

	return c
}

// renderTriggerHero is the human-format response for `build trigger` —
// a short success line plus the URL on its own line. The full build
// detail view would be misleading here because most fields aren't
// returned by the trigger response (the build hasn't run yet).
func renderTriggerHero(w io.Writer, b internalbuild.Build) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)

	headline := strings.Builder{}
	headline.WriteString(s.Success.Render("✓"))
	headline.WriteString(" ")
	headline.WriteString(s.Bold.Render("Build triggered"))
	if b.BuildNumber > 0 {
		headline.WriteString(s.Dim.Render(fmt.Sprintf("  #%d", b.BuildNumber)))
	}
	if b.Workflow != "" {
		headline.WriteString("  ")
		headline.WriteString(b.Workflow)
	}
	switch {
	case b.Branch != "":
		headline.WriteString(s.Dim.Render(" on "))
		headline.WriteString(b.Branch)
	case b.Tag != "":
		headline.WriteString(s.Dim.Render(" tag "))
		headline.WriteString(b.Tag)
	}
	ew.F("%s\n", headline.String())
	if b.BuildURL != "" {
		ew.F("  %s\n", s.URL.Render(b.BuildURL))
	}
	return ew.Err
}
