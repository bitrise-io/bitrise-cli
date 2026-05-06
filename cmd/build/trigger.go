package build

import (
	"encoding/json"
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
  --wait                 stream logs and wait for the build to finish; exits 0 on success,
                         1 on failure. With --output json the final build record is written
                         to stdout and logs go to stderr.
  --interval DURATION    log polling interval when --wait is set (default 3s)`,
		Example: `  bitrise-cli build trigger --app my-app-slug --workflow primary
  bitrise-cli build trigger --app my-app-slug --workflow deploy --branch release/1.2 --output json
  bitrise-cli build trigger --app my-app-slug --pipeline my-pipeline --branch main
  bitrise-cli build trigger --app my-app-slug --workflow primary --tag v1.2.3
  bitrise-cli build trigger --app my-app-slug --workflow primary --env '{"MY_VAR":"hello","OTHER":"world"}'
  bitrise-cli build trigger --app my-app-slug --workflow primary --wait
  bitrise-cli build trigger --app my-app-slug --workflow primary --wait --output json`,
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

			if !wait {
				return output.Render(cmd.OutOrStdout(), format, b, renderTriggerHero)
			}

			// --wait: stream logs then exit with a code reflecting the build outcome.
			// In JSON mode logs go to stderr so stdout carries only the final build JSON.
			logWriter := io.Writer(cmd.OutOrStdout())
			if format == output.JSON {
				logWriter = cmd.ErrOrStderr()
			}
			return runWatch(cmd, svc, b, interval, logWriter, format)
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
	c.Flags().BoolVar(&wait, "wait", false, "stream logs and wait for the build to finish (exit code reflects build outcome)")
	c.Flags().DurationVar(&interval, "interval", 3*time.Second, "log polling interval when --wait is set")
	c.MarkFlagsMutuallyExclusive("workflow", "pipeline")

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
