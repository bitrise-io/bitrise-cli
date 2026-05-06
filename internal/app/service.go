// Package app holds the business-logic layer for app and workflow operations.
//
// Mirrors internal/build: cobra-free, returns canned data today, will hold
// a *bitriseapi.Client once we wire real API calls.
package app

import (
	"context"
	"fmt"
)

// App represents a registered Bitrise app (project).
type App struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Provider    string `json:"provider"`
	RepoURL     string `json:"repo_url"`
	OwnerType   string `json:"owner_type,omitempty"`
	OwnerSlug   string `json:"owner_slug,omitempty"`
	ProjectType string `json:"project_type,omitempty"`
	IsDisabled  bool   `json:"is_disabled,omitempty"`
}

// Workflow is a workflow defined on an app's bitrise.yml.
type Workflow struct {
	ID string `json:"id"`
}

// ListOptions paginates app and workflow lists.
type ListOptions struct {
	Limit  int
	Cursor string
}

// AppsResult is one page of apps.
type AppsResult struct {
	Items      []App  `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// WorkflowsResult is the set of workflows on an app.
type WorkflowsResult struct {
	Items []Workflow `json:"items"`
}

// Service exposes app and workflow operations to the cmd layer.
type Service struct{}

// NewService returns a stub Service.
func NewService() *Service { return &Service{} }

// List returns a canned page of apps.
func (s *Service) List(_ context.Context, _ ListOptions) (AppsResult, error) {
	return AppsResult{
		Items: []App{
			{
				Slug:        "stub-app-aaa",
				Title:       "android-sample",
				Provider:    "github",
				RepoURL:     "https://github.com/bitrise-io/android-sample",
				OwnerType:   "Organization",
				OwnerSlug:   "bitrise-io",
				ProjectType: "android",
			},
			{
				Slug:        "stub-app-bbb",
				Title:       "ios-sample",
				Provider:    "github",
				RepoURL:     "https://github.com/bitrise-io/ios-sample",
				OwnerType:   "Organization",
				OwnerSlug:   "bitrise-io",
				ProjectType: "ios",
			},
			{
				Slug:        "stub-app-ccc",
				Title:       "legacy-flutter",
				Provider:    "gitlab",
				RepoURL:     "https://gitlab.com/example/legacy-flutter",
				OwnerType:   "User",
				OwnerSlug:   "example",
				ProjectType: "flutter",
				IsDisabled:  true,
			},
		},
	}, nil
}

// View returns details of a single app by slug.
func (s *Service) View(_ context.Context, appSlug string) (App, error) {
	if appSlug == "" {
		return App{}, fmt.Errorf("app slug is required")
	}
	return App{
		Slug:        appSlug,
		Title:       "android-sample",
		Provider:    "github",
		RepoURL:     "https://github.com/bitrise-io/android-sample",
		OwnerType:   "Organization",
		OwnerSlug:   "bitrise-io",
		ProjectType: "android",
	}, nil
}

// ListWorkflows returns the workflows defined on an app.
func (s *Service) ListWorkflows(_ context.Context, appSlug string) (WorkflowsResult, error) {
	if appSlug == "" {
		return WorkflowsResult{}, fmt.Errorf("app slug is required")
	}
	return WorkflowsResult{
		Items: []Workflow{
			{ID: "primary"},
			{ID: "deploy"},
			{ID: "nightly"},
		},
	}, nil
}
