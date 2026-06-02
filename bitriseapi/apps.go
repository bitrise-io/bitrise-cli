package bitriseapi

import (
	"context"
	"net/url"
	"strconv"
)

// App represents a Bitrise app (project), as returned by GET /apps and
// GET /apps/{slug}. Field names match v0.AppResponseItemModel in the
// Bitrise API swagger.
type App struct {
	Slug                  string   `json:"slug"`
	Title                 string   `json:"title"`
	Provider              string   `json:"provider"`
	RepoOwner             string   `json:"repo_owner"`
	RepoSlug              string   `json:"repo_slug"`
	RepoURL               string   `json:"repo_url"`
	ProjectType           string   `json:"project_type"`
	ProjectID             string   `json:"project_id,omitempty"`
	Status                int      `json:"status"`
	IsDisabled            bool     `json:"is_disabled"`
	IsGitHubChecksEnabled bool     `json:"is_github_checks_enabled,omitempty"`
	IsPublic              bool     `json:"is_public,omitempty"`
	AvatarURL             string   `json:"avatar_url,omitempty"`
	Owner                 AppOwner `json:"owner"`
}

// AppOwner is the workspace or user that owns the app
// (v0.OwnerAccountResponseModel).
type AppOwner struct {
	AccountType string `json:"account_type"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
}

// AppsListOptions filters and paginates the GET /apps endpoint. All fields
// are optional.
type AppsListOptions struct {
	// SortBy controls ordering: "created_at" or "last_build_at" per the API.
	SortBy string
	// Next is the pagination cursor (slug of the first app for the next page).
	Next string
	// Limit is the max items per page; the server default is 50.
	Limit int
	// Title filters apps by title.
	Title string
	// ProjectType filters by project type, e.g. "ios", "android".
	ProjectType string
}

func (o AppsListOptions) params() url.Values {
	p := url.Values{}
	if o.SortBy != "" {
		p.Set("sort_by", o.SortBy)
	}
	if o.Next != "" {
		p.Set("next", o.Next)
	}
	if o.Limit > 0 {
		p.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Title != "" {
		p.Set("title", o.Title)
	}
	if o.ProjectType != "" {
		p.Set("project_type", o.ProjectType)
	}
	return p
}

// Apps returns one page of apps the authenticated user can access.
// Endpoint: GET /apps.
func (c *Client) Apps(ctx context.Context, opts AppsListOptions) (Page[App], error) {
	return getPage[App](ctx, c, "/apps", opts.params())
}

// App returns the details of a single app by slug.
// Endpoint: GET /apps/{app-slug}.
func (c *Client) App(ctx context.Context, appSlug string) (App, error) {
	return get[App](ctx, c, "/apps/"+appSlug, nil)
}
