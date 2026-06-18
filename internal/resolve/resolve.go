// Package resolve maps user-supplied names and IDs to canonical API slugs.
// Both app titles and workspace names are resolved through a targeted API
// query; results are stored in a name cache so repeated invocations with the
// same name skip the network call.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	"github.com/bitrise-io/bitrise-cli/internal/cache"
	"github.com/bitrise-io/bitrise-cli/internal/config"
)

// Resolver maps display names to API slugs for apps and workspaces.
type Resolver struct {
	client *bitriseapi.Client
	cache  *cache.Cache
}

// New returns a Resolver backed by the given API client and cache.
// cache may be nil — resolution still works, just without in-memory caching.
func New(client *bitriseapi.Client, c *cache.Cache) *Resolver {
	return &Resolver{client: client, cache: c}
}

// AppSlug maps value to an app slug. See ResolveApp for resolution semantics.
func (r *Resolver) AppSlug(ctx context.Context, value string) (string, error) {
	app, _, err := r.ResolveApp(ctx, value)
	return app.Slug, err
}

// WorkspaceSlug maps value to a workspace slug. A targeted GET
// /organizations call is used; matching is exact and case-insensitive. On zero
// name matches the value is returned as-is (literal-slug passthrough). On 2+
// matches an ambiguity error is returned.
func (r *Resolver) WorkspaceSlug(ctx context.Context, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if r.cache != nil {
		if slug, ok := r.cache.LookupWorkspace(value); ok {
			return slug, nil
		}
	}
	orgs, err := r.client.Organizations(ctx)
	if err != nil {
		return "", fmt.Errorf("look up workspace %q: %w", value, err)
	}
	var matches []bitriseapi.Organization
	for _, o := range orgs {
		if strings.EqualFold(o.Name, value) {
			matches = append(matches, o)
		}
	}
	switch len(matches) {
	case 0:
		return value, nil // no name match → treat as literal slug
	case 1:
		if r.cache != nil {
			r.cache.SetWorkspace(matches[0].Name, matches[0].Slug)
		}
		return matches[0].Slug, nil
	default:
		slugs := make([]string, 0, len(matches))
		for _, m := range matches {
			slugs = append(slugs, m.Slug)
		}
		return "", fmt.Errorf("workspace name %q is ambiguous (matches %d workspaces: %v) — pass a workspace ID instead", value, len(matches), slugs)
	}
}

// SoleWorkspace returns the user's workspace when they have exactly one. With
// zero or 2+ workspaces it returns a friendly error, since no default can be
// picked unambiguously. This is the single definition of the "exactly one
// workspace" rule and its guidance message: both the --workspace fallback
// (cmdutil.ResolveWorkspaceID) and `app create`'s auto-detect route through it.
func SoleWorkspace(orgs []bitriseapi.Organization) (bitriseapi.Organization, error) {
	switch len(orgs) {
	case 0:
		return bitriseapi.Organization{}, errors.New("no workspaces found for this account — create one in the Bitrise dashboard, or pass --workspace")
	case 1:
		return orgs[0], nil
	default:
		slugs := make([]string, 0, len(orgs))
		for _, o := range orgs {
			slugs = append(slugs, o.Slug)
		}
		sort.Strings(slugs)
		return bitriseapi.Organization{}, fmt.Errorf("multiple workspaces available — pass --workspace, set %s, or run 'bitrise-cli config set %s <id>'. Available: %s",
			config.EnvWorkspaceID, config.KeyDefaultWorkspaceID, strings.Join(slugs, ", "))
	}
}

// DefaultWorkspace fetches the user's workspaces (GET /organizations) and
// returns the only one. See SoleWorkspace for the zero/multiple cases.
func (r *Resolver) DefaultWorkspace(ctx context.Context) (bitriseapi.Organization, error) {
	orgs, err := r.client.Organizations(ctx)
	if err != nil {
		return bitriseapi.Organization{}, fmt.Errorf("list workspaces: %w", err)
	}
	return SoleWorkspace(orgs)
}

// ResolveApp is like AppSlug but returns the full bitriseapi.App when value
// was matched by a name query, so the caller can skip a second GET /apps/{slug}.
// complete=false means value was not resolved by name: app.Slug holds the
// resolved slug (from cache or passthrough) and the caller must fetch full data.
func (r *Resolver) ResolveApp(ctx context.Context, value string) (app bitriseapi.App, complete bool, err error) {
	if value == "" {
		return bitriseapi.App{}, false, nil
	}
	if r.cache != nil {
		if slug, ok := r.cache.LookupApp(value); ok {
			return bitriseapi.App{Slug: slug}, false, nil
		}
	}
	page, err := r.client.Apps(ctx, bitriseapi.AppsListOptions{Title: value})
	if err != nil {
		return bitriseapi.App{}, false, fmt.Errorf("look up app %q: %w", value, err)
	}
	var matches []bitriseapi.App
	for _, a := range page.Items {
		if strings.EqualFold(a.Title, value) {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return bitriseapi.App{Slug: value}, false, nil
	case 1:
		if r.cache != nil {
			r.cache.SetApp(matches[0].Title, matches[0].Slug)
		}
		return matches[0], true, nil
	default:
		slugs := make([]string, 0, len(matches))
		for _, m := range matches {
			slugs = append(slugs, m.Slug)
		}
		return bitriseapi.App{}, false, fmt.Errorf("app name %q is ambiguous (matches %d apps: %v) — pass an app ID instead", value, len(matches), slugs)
	}
}
