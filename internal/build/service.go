// Package build holds the business-logic layer for build operations.
//
// All methods call the Bitrise API via the bitriseapi client.
package build

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

// Build is the CLI-facing build record. JSON tags define the stable
// `--output json` shape; rename fields with care.
type Build struct {
	Slug                    string     `json:"slug"`
	AppSlug                 string     `json:"app_slug"`
	BuildNumber             int        `json:"build_number"`
	Status                  string     `json:"status"`
	StatusText              string     `json:"status_text,omitempty"`
	AbortReason             string     `json:"abort_reason,omitempty"`
	IsOnHold                bool       `json:"is_on_hold,omitempty"`
	Rebuildable             bool       `json:"rebuildable,omitempty"`
	Workflow                string     `json:"workflow,omitempty"`
	PipelineWorkflowID      string     `json:"pipeline_workflow_id,omitempty"`
	Branch                  string     `json:"branch,omitempty"`
	Tag                     string     `json:"tag,omitempty"`
	PullRequestID           int        `json:"pull_request_id,omitempty"`
	PullRequestTargetBranch string     `json:"pull_request_target_branch,omitempty"`
	PullRequestViewURL      string     `json:"pull_request_view_url,omitempty"`
	CommitHash              string     `json:"commit_hash,omitempty"`
	CommitMessage           string     `json:"commit_message,omitempty"`
	TriggeredAt             time.Time  `json:"triggered_at,omitempty"`
	TriggeredBy             string     `json:"triggered_by,omitempty"`
	FinishedAt              *time.Time `json:"finished_at,omitempty"`
	StackIdentifier         string     `json:"stack_identifier,omitempty"`
	MachineTypeID           string     `json:"machine_type_id,omitempty"`
	CreditCost              int        `json:"credit_cost,omitempty"`
	BuildURL                string     `json:"build_url,omitempty"`
}

// TriggerEnv is an environment variable to inject into a triggered build.
type TriggerEnv struct {
	Key   string
	Value string
}

// TriggerRequest describes a build to start.
type TriggerRequest struct {
	AppSlug       string
	Workflow      string
	Pipeline      string
	Branch        string
	BranchDest    string
	Tag           string
	CommitHash    string
	CommitMessage string
	PullRequestID int
	Priority      int
	Environments  []TriggerEnv
}

// ListOptions filters and paginates build lists. Status is a CLI-friendly
// string ("success", "failed", "aborted", "aborted-with-success",
// "in-progress"); the service translates it to the API's integer value.
// After/Before are optional time bounds; the service converts them to unix timestamps.
// IsPipelineBuild is a tri-state: nil = no filter, true = pipeline builds only, false = non-pipeline only.
type ListOptions struct {
	AppSlug          string
	Branch           string
	Workflow         string
	CommitMessage    string
	TriggerEventType string
	PullRequestID    int
	BuildNumber      int
	After            *time.Time
	Before           *time.Time
	Status           string
	SortBy           string
	IsPipelineBuild  *bool
	Limit            int
	Cursor           string
}

// ListResult is one page of builds plus the cursor for the next page.
type ListResult struct {
	Items      []Build `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// Service exposes build operations to the cmd layer.
type Service struct {
	client *bitriseapi.Client
}

// NewService returns a Service backed by the given API client. The client
// must be non-nil — every method makes a network call.
func NewService(client *bitriseapi.Client) *Service {
	return &Service{client: client}
}

// Trigger starts a new build for the given app + workflow.
// Endpoint: POST /apps/{app-slug}/builds.
func (s *Service) Trigger(ctx context.Context, req TriggerRequest) (Build, error) {
	if s.client == nil {
		return Build{}, fmt.Errorf("API client not configured")
	}
	if req.AppSlug == "" {
		return Build{}, fmt.Errorf("app slug is required")
	}
	envs := make([]bitriseapi.BuildTriggerEnv, 0, len(req.Environments))
	for _, e := range req.Environments {
		envs = append(envs, bitriseapi.BuildTriggerEnv{MappedTo: e.Key, Value: e.Value, IsExpand: true})
	}
	resp, err := s.client.TriggerBuild(ctx, req.AppSlug, bitriseapi.BuildTriggerParams{
		HookInfo: bitriseapi.BuildTriggerHookInfo{Type: "bitrise"},
		BuildParams: bitriseapi.BuildTriggerBuildParams{
			WorkflowID:    req.Workflow,
			PipelineID:    req.Pipeline,
			Branch:        req.Branch,
			BranchDest:    req.BranchDest,
			Tag:           req.Tag,
			CommitHash:    req.CommitHash,
			CommitMessage: req.CommitMessage,
			PullRequestID: req.PullRequestID,
			Priority:      req.Priority,
			Environments:  envs,
		},
	})
	if err != nil {
		return Build{}, err
	}
	return triggerRespToBuild(resp, req), nil
}

// List returns one page of builds for an app.
// Endpoint: GET /apps/{app-slug}/builds.
func (s *Service) List(ctx context.Context, opts ListOptions) (ListResult, error) {
	if s.client == nil {
		return ListResult{}, fmt.Errorf("API client not configured")
	}
	if opts.AppSlug == "" {
		return ListResult{}, fmt.Errorf("app slug is required")
	}
	statusInt, err := parseStatusFilter(opts.Status)
	if err != nil {
		return ListResult{}, err
	}
	apiOpts := bitriseapi.BuildsListOptions{
		SortBy:           opts.SortBy,
		Branch:           opts.Branch,
		Workflow:         opts.Workflow,
		CommitMessage:    opts.CommitMessage,
		TriggerEventType: opts.TriggerEventType,
		PullRequestID:    opts.PullRequestID,
		BuildNumber:      opts.BuildNumber,
		Status:           statusInt,
		IsPipelineBuild:  opts.IsPipelineBuild,
		Limit:            opts.Limit,
		Next:             opts.Cursor,
	}
	if opts.After != nil {
		apiOpts.After = int(opts.After.Unix())
	}
	if opts.Before != nil {
		apiOpts.Before = int(opts.Before.Unix())
	}
	page, err := s.client.Builds(ctx, opts.AppSlug, apiOpts)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]Build, 0, len(page.Items))
	for _, b := range page.Items {
		items = append(items, fromAPI(b, opts.AppSlug))
	}
	return ListResult{Items: items, NextCursor: page.Paging.Next}, nil
}

// View returns details for a single build.
// Endpoint: GET /apps/{app-slug}/builds/{build-slug}.
func (s *Service) View(ctx context.Context, appSlug, buildSlug string) (Build, error) {
	if s.client == nil {
		return Build{}, fmt.Errorf("API client not configured")
	}
	if appSlug == "" {
		return Build{}, fmt.Errorf("app slug is required")
	}
	if buildSlug == "" {
		return Build{}, fmt.Errorf("build slug is required")
	}
	b, err := s.client.Build(ctx, appSlug, buildSlug)
	if err != nil {
		return Build{}, err
	}
	return fromAPI(b, appSlug), nil
}

// Log streams the build log for the given build to w. For finished
// builds the full archived log is streamed; for in-progress builds the
// chunks available so far are written.
// Endpoint: GET /apps/{app-slug}/builds/{build-slug}/log.
func (s *Service) Log(ctx context.Context, appSlug, buildSlug string, w io.Writer) error {
	if s.client == nil {
		return fmt.Errorf("API client not configured")
	}
	if appSlug == "" {
		return fmt.Errorf("app slug is required")
	}
	if buildSlug == "" {
		return fmt.Errorf("build slug is required")
	}
	_, err := s.client.BuildLog(ctx, appSlug, buildSlug, w)
	return err
}

// fromAPI maps a bitriseapi.Build (wire shape) into the CLI's Build type.
// appSlug is taken from the request context because the API response
// doesn't echo it back.
func fromAPI(b bitriseapi.Build, appSlug string) Build {
	out := Build{
		Slug:                    b.Slug,
		AppSlug:                 appSlug,
		BuildNumber:             b.BuildNumber,
		Status:                  statusString(b.Status),
		StatusText:              b.StatusText,
		AbortReason:             b.AbortReason,
		IsOnHold:                b.IsOnHold,
		Rebuildable:             b.Rebuildable,
		Workflow:                b.TriggeredWorkflow,
		PipelineWorkflowID:      b.PipelineWorkflowID,
		Branch:                  b.Branch,
		Tag:                     b.Tag,
		PullRequestID:           b.PullRequestID,
		PullRequestTargetBranch: b.PullRequestTargetBranch,
		PullRequestViewURL:      b.PullRequestViewURL,
		CommitHash:              b.CommitHash,
		CommitMessage:           b.CommitMessage,
		TriggeredBy:             b.TriggeredBy,
		StackIdentifier:         b.StackIdentifier,
		MachineTypeID:           b.MachineTypeID,
		CreditCost:              b.CreditCost,
	}
	if !b.TriggeredAt.IsZero() {
		out.TriggeredAt = b.TriggeredAt.UTC()
	}
	if !b.FinishedAt.IsZero() {
		t := b.FinishedAt.UTC()
		out.FinishedAt = &t
	}
	return out
}

// triggerRespToBuild collapses the trigger response into our Build shape,
// preferring the modern Results[0] over the deprecated top-level fields.
func triggerRespToBuild(resp bitriseapi.BuildTriggerResp, req TriggerRequest) Build {
	slug := resp.BuildSlug
	number := resp.BuildNumber
	url := resp.BuildURL
	workflow := resp.TriggeredWorkflow
	if len(resp.Results) > 0 {
		r := resp.Results[0]
		if r.BuildSlug != "" {
			slug = r.BuildSlug
		}
		if r.BuildNumber != 0 {
			number = r.BuildNumber
		}
		if r.BuildURL != "" {
			url = r.BuildURL
		}
		if r.TriggeredWorkflow != "" {
			workflow = r.TriggeredWorkflow
		}
	}
	if workflow == "" {
		workflow = req.Workflow
	}
	return Build{
		Slug:          slug,
		AppSlug:       req.AppSlug,
		BuildNumber:   number,
		Status:        "in-progress",
		StatusText:    resp.Message,
		Branch:        req.Branch,
		Tag:           req.Tag,
		Workflow:      workflow,
		CommitHash:    req.CommitHash,
		CommitMessage: req.CommitMessage,
		TriggeredAt:   time.Now().UTC(),
		BuildURL:      url,
	}
}

// statusString translates the API's integer status into a stable string
// for `--output json`. Unknown values fall through as the integer for
// forward-compat with new statuses.
func statusString(n int) string {
	switch n {
	case 0:
		return "in-progress"
	case 1:
		return "success"
	case 2:
		return "failed"
	case 3:
		return "aborted"
	case 4:
		return "aborted-with-success"
	default:
		return strconv.Itoa(n)
	}
}

// parseStatusFilter is the inverse of statusString. Returns nil when no
// filter is requested. Returns an error for unknown values so users see
// a quick message instead of a silent passthrough.
func parseStatusFilter(s string) (*int, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // intentional — nil pointer signals "no filter"
	}
	var n int
	switch s {
	case "in-progress":
		n = 0
	case "success":
		n = 1
	case "failed":
		n = 2
	case "aborted":
		n = 3
	case "aborted-with-success":
		n = 4
	default:
		return nil, fmt.Errorf("unknown build status %q (expected: in-progress, success, failed, aborted, aborted-with-success)", s)
	}
	return &n, nil
}
