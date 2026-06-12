// Package app holds the business-logic layer for app and workflow operations.
//
// All methods call the Bitrise API via the bitriseapi client.
package app

import (
	"context"
	"fmt"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	"github.com/bitrise-io/bitrise-cli/internal/resolve"
)

// App represents a registered Bitrise app (project), trimmed to the fields
// the CLI surfaces. The full set of fields is on bitriseapi.App.
type App struct {
	Slug        string `json:"id"`
	Title       string `json:"title"`
	Provider    string `json:"provider"`
	RepoURL     string `json:"repo_url"`
	OwnerSlug   string `json:"workspace_id,omitempty"`
	ProjectType string `json:"project_type,omitempty"`
	IsDisabled  bool   `json:"is_disabled,omitempty"`
}

// ListOptions paginates and filters app lists. Filter fields map to the
// query parameters of GET /apps.
type ListOptions struct {
	Limit       int
	Cursor      string
	SortBy      string
	Title       string
	ProjectType string
}

// AppsResult is one page of apps.
type AppsResult struct {
	Items      []App  `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Service exposes app and workflow operations to the cmd layer.
type Service struct {
	client *bitriseapi.Client
}

// NewService returns a Service backed by the given API client. The client
// must be non-nil — every method in this Service makes a network call.
func NewService(client *bitriseapi.Client) *Service {
	return &Service{client: client}
}

// List returns one page of apps the authenticated user can access by
// calling GET /apps on the Bitrise API.
func (s *Service) List(ctx context.Context, opts ListOptions) (AppsResult, error) {
	if s.client == nil {
		return AppsResult{}, fmt.Errorf("API client not configured")
	}
	page, err := s.client.Apps(ctx, bitriseapi.AppsListOptions{
		SortBy:      opts.SortBy,
		Next:        opts.Cursor,
		Limit:       opts.Limit,
		Title:       opts.Title,
		ProjectType: opts.ProjectType,
	})
	if err != nil {
		return AppsResult{}, err
	}
	items := make([]App, 0, len(page.Items))
	for _, a := range page.Items {
		items = append(items, fromAPI(a))
	}
	return AppsResult{
		Items:      items,
		NextCursor: page.Paging.Next,
	}, nil
}

// fromAPI maps the wire-format bitriseapi.App into the trimmed CLI shape.
func fromAPI(a bitriseapi.App) App {
	return App{
		Slug:        a.Slug,
		Title:       a.Title,
		Provider:    a.Provider,
		RepoURL:     a.RepoURL,
		OwnerSlug:   a.Owner.Slug,
		ProjectType: a.ProjectType,
		IsDisabled:  a.IsDisabled,
	}
}

// ViewByNameOrSlug resolves value via r and returns the app. When r found a
// name match in the current API call (complete=true), that result is used
// directly — no second request. When value passes through as a literal slug
// (cache hit or no name match), GET /apps/{slug} is called.
func (s *Service) ViewByNameOrSlug(ctx context.Context, r *resolve.Resolver, value string) (App, error) {
	app, complete, err := r.ResolveApp(ctx, value)
	if err != nil {
		return App{}, err
	}
	if complete {
		return fromAPI(app), nil
	}
	return s.View(ctx, app.Slug)
}

// View returns details of a single app by slug.
// Endpoint: GET /apps/{app-slug}.
func (s *Service) View(ctx context.Context, appSlug string) (App, error) {
	if s.client == nil {
		return App{}, fmt.Errorf("API client not configured")
	}
	if appSlug == "" {
		return App{}, fmt.Errorf("app ID is required")
	}
	app, err := s.client.App(ctx, appSlug)
	if err != nil {
		return App{}, err
	}
	return fromAPI(app), nil
}
