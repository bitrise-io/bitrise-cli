// Package build holds the business-logic layer for build operations.
//
// The service is intentionally decoupled from any presentation concern
// (cobra, flag parsing, output formatting). Today it returns canned
// data; once we wire the bitriseapi client through, only this layer
// changes — the cmd handlers stay the same.
package build

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Build represents a single CI build. The JSON tags define the stable
// shape emitted by `--output json`, so rename fields with care.
type Build struct {
	Slug          string     `json:"slug"`
	AppSlug       string     `json:"app_slug"`
	BuildNumber   int        `json:"build_number"`
	Status        string     `json:"status"`
	StatusText    string     `json:"status_text,omitempty"`
	Branch        string     `json:"branch"`
	Workflow      string     `json:"workflow"`
	CommitHash    string     `json:"commit_hash,omitempty"`
	CommitMessage string     `json:"commit_message,omitempty"`
	TriggeredAt   time.Time  `json:"triggered_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	BuildURL      string     `json:"build_url,omitempty"`
}

// TriggerRequest describes a build to start.
type TriggerRequest struct {
	AppSlug       string
	Workflow      string
	Branch        string
	CommitHash    string
	CommitMessage string
}

// ListOptions filters and paginates build lists.
type ListOptions struct {
	AppSlug  string
	Branch   string
	Workflow string
	Status   string
	Limit    int
	Cursor   string
}

// ListResult is one page of builds plus the cursor for the next page.
type ListResult struct {
	Items      []Build `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// Service exposes build operations to the cmd layer. It will eventually
// be backed by *bitriseapi.Client; today it returns stub data so the
// CLI surface can be exercised end-to-end without network calls.
type Service struct{}

// NewService returns a stub Service.
func NewService() *Service { return &Service{} }

// Trigger pretends to start a build and returns a fake "in-progress" record.
func (s *Service) Trigger(_ context.Context, req TriggerRequest) (Build, error) {
	if req.AppSlug == "" {
		return Build{}, fmt.Errorf("app slug is required")
	}
	if req.Workflow == "" {
		return Build{}, fmt.Errorf("workflow is required")
	}
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	now := time.Now().UTC()
	return Build{
		Slug:          "stub-" + req.Workflow + "-001",
		AppSlug:       req.AppSlug,
		BuildNumber:   42,
		Status:        "in-progress",
		StatusText:    "Build started (stub)",
		Branch:        branch,
		Workflow:      req.Workflow,
		CommitHash:    req.CommitHash,
		CommitMessage: req.CommitMessage,
		TriggeredAt:   now,
		BuildURL:      fmt.Sprintf("https://app.bitrise.io/build/stub-%s-001", req.Workflow),
	}, nil
}

// List returns a stub page of builds for the given app.
func (s *Service) List(_ context.Context, opts ListOptions) (ListResult, error) {
	if opts.AppSlug == "" {
		return ListResult{}, fmt.Errorf("app slug is required")
	}
	now := time.Now().UTC()
	finished := now.Add(-10 * time.Minute)
	return ListResult{
		Items: []Build{
			{
				Slug:        "stub-build-aaa",
				AppSlug:     opts.AppSlug,
				BuildNumber: 42,
				Status:      "in-progress",
				Branch:      "main",
				Workflow:    "primary",
				TriggeredAt: now.Add(-2 * time.Minute),
				BuildURL:    "https://app.bitrise.io/build/stub-build-aaa",
			},
			{
				Slug:        "stub-build-bbb",
				AppSlug:     opts.AppSlug,
				BuildNumber: 41,
				Status:      "success",
				Branch:      "main",
				Workflow:    "primary",
				TriggeredAt: now.Add(-30 * time.Minute),
				FinishedAt:  &finished,
				BuildURL:    "https://app.bitrise.io/build/stub-build-bbb",
			},
			{
				Slug:        "stub-build-ccc",
				AppSlug:     opts.AppSlug,
				BuildNumber: 40,
				Status:      "failed",
				Branch:      "feature/x",
				Workflow:    "deploy",
				TriggeredAt: now.Add(-2 * time.Hour),
				FinishedAt:  &finished,
				BuildURL:    "https://app.bitrise.io/build/stub-build-ccc",
			},
		},
	}, nil
}

// View returns details for a single build.
func (s *Service) View(_ context.Context, appSlug, buildSlug string) (Build, error) {
	if appSlug == "" {
		return Build{}, fmt.Errorf("app slug is required")
	}
	if buildSlug == "" {
		return Build{}, fmt.Errorf("build slug is required")
	}
	now := time.Now().UTC()
	finished := now.Add(-1 * time.Minute)
	return Build{
		Slug:          buildSlug,
		AppSlug:       appSlug,
		BuildNumber:   42,
		Status:        "success",
		StatusText:    "Build succeeded (stub)",
		Branch:        "main",
		Workflow:      "primary",
		CommitHash:    "deadbeefcafef00d",
		CommitMessage: "stub: example commit",
		TriggeredAt:   now.Add(-5 * time.Minute),
		FinishedAt:    &finished,
		BuildURL:      fmt.Sprintf("https://app.bitrise.io/build/%s", buildSlug),
	}, nil
}

// Log streams the build log for the given build to w.
// In stub mode it writes a few canned lines.
func (s *Service) Log(_ context.Context, appSlug, buildSlug string, w io.Writer) error {
	if appSlug == "" {
		return fmt.Errorf("app slug is required")
	}
	if buildSlug == "" {
		return fmt.Errorf("build slug is required")
	}
	lines := []string{
		fmt.Sprintf("[stub] log for app=%s build=%s", appSlug, buildSlug),
		"[stub] step 1/3: git-clone        ✓",
		"[stub] step 2/3: yarn-install     ✓",
		"[stub] step 3/3: deploy-to-bitrise ✓",
		"[stub] build finished",
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
