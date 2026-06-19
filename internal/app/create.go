package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strings"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
	"github.com/bitrise-io/bitrise-cli/internal/resolve"
)

// DefaultStackID is the stack used by `app create` when --stack is not
// supplied. Any valid stack ID for the org will do — this one is broadly
// available — and users can override with --stack.
const DefaultStackID = "linux-docker-android-22.04"

// DefaultProjectType is the project_type sent to /finish when --project-type
// is not supplied. "other" is a safe minimal preset.
const DefaultProjectType = "other"

// DefaultProvider is the provider sent to /apps/register when --provider is
// not supplied. The website's "Add new app" flow sends "custom" regardless
// of host; this matches that behavior so we don't trip over GitHub-App-vs-
// GitHub permission checks. Override with --provider github (etc.) when the
// repo is linked to Bitrise via the GitHub App.
const DefaultProvider = "custom"

// FlowTypeCLI is the analytics attribution sent on register/finish so the
// server can distinguish CLI-driven app creation from website-driven flows.
const FlowTypeCLI = "cli"

// DefaultBranchFallback is the branch name sent to /apps/register when none
// can be detected from the local git checkout.
const DefaultBranchFallback = "master"

// CreateOptions are the inputs for Service.Create. Empty fields trigger
// auto-detection (git, single-org pick) where applicable; required fields
// produce an error if the auto-detection can't fill them in.
type CreateOptions struct {
	RepoURL     string
	Branch      string
	Title       string
	Provider    string // "auto" or one of the API's accepted values
	OrgSlug     string // empty → fall back to single-org auto-detect
	StackID     string
	ProjectType string
	Public      bool

	// BitriseYML is the YAML to upload after /finish. Empty means skip the
	// upload and let the server preset chosen by ProjectType take effect.
	BitriseYML string
}

// CreateResult is what Service.Create returns once the app is registered
// and finished. BitriseYMLUploaded is true when step 3 of the flow ran.
type CreateResult struct {
	Slug              string `json:"id"`
	Title             string `json:"title"`
	RepoURL           string `json:"repo_url"`
	DefaultBranch     string `json:"default_branch"`
	BuildTriggerToken string `json:"build_trigger_token"`

	OrgSlug            string `json:"workspace_id"`
	StackID            string `json:"stack_id"`
	ProjectType        string `json:"project_type"`
	BitriseYMLUploaded bool   `json:"bitrise_yml_uploaded"`
}

// Create runs the register → finish → (optional) upload sequence on the
// Bitrise API and returns the new app's identifying details.
//
// Required: opts.RepoURL (or detectable git remote), opts.Provider (or
// "auto"), and an organization (via opts.OrgSlug or single-org detection).
// Defaults are applied for Branch, Title, StackID, ProjectType.
func (s *Service) Create(ctx context.Context, opts CreateOptions) (CreateResult, error) {
	if s.client == nil {
		return CreateResult{}, fmt.Errorf("API client not configured")
	}

	if opts.RepoURL == "" {
		return CreateResult{}, errors.New("repo URL is required (pass --repo-url or run inside a git repo with an 'origin' remote)")
	}
	provider, err := resolveProvider(opts.Provider, opts.RepoURL)
	if err != nil {
		return CreateResult{}, err
	}

	branch := opts.Branch
	if branch == "" {
		branch = DefaultBranchFallback
	}
	title := opts.Title
	if title == "" {
		title = deriveTitle(opts.RepoURL)
	}
	stackID := opts.StackID
	if stackID == "" {
		stackID = DefaultStackID
	}
	projectType := opts.ProjectType
	if projectType == "" {
		projectType = DefaultProjectType
	}

	orgSlug := opts.OrgSlug
	if orgSlug == "" {
		orgSlug, err = s.autoDetectOrg(ctx)
		if err != nil {
			return CreateResult{}, err
		}
	}

	reg, err := s.client.RegisterApp(ctx, bitriseapi.RegisterAppRequest{
		RepoURL:           opts.RepoURL,
		OrganizationSlug:  orgSlug,
		Provider:          provider,
		IsPublic:          opts.Public,
		Title:             title,
		DefaultBranchName: branch,
		FlowType:          FlowTypeCLI,
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("register app: %w", err)
	}

	fin, err := s.client.FinishApp(ctx, reg.Slug, bitriseapi.FinishAppRequest{
		StackID:     stackID,
		Mode:        "manual",
		ProjectType: projectType,
		Config:      configIDForProjectType(projectType),
		FlowType:    FlowTypeCLI,
	})
	if err != nil {
		return CreateResult{}, fmt.Errorf("finish app %s: %w", reg.Slug, err)
	}

	res := CreateResult{
		Slug:              reg.Slug,
		Title:             title,
		RepoURL:           opts.RepoURL,
		DefaultBranch:     fin.BranchName,
		BuildTriggerToken: fin.BuildTriggerToken,
		OrgSlug:           orgSlug,
		StackID:           stackID,
		ProjectType:       projectType,
	}
	if res.DefaultBranch == "" {
		res.DefaultBranch = branch
	}

	if opts.BitriseYML != "" {
		if err := s.client.UploadAppConfig(ctx, reg.Slug, opts.BitriseYML); err != nil {
			return res, fmt.Errorf("upload bitrise.yml: %w", err)
		}
		res.BitriseYMLUploaded = true
	}

	return res, nil
}

// autoDetectOrg fetches the user's organizations and returns the slug
// when there's exactly one. 0 or 2+ orgs produce a friendly error. The
// "exactly one workspace" rule itself lives in resolve.SoleWorkspace so the
// CLI applies it identically here and in the --workspace fallback.
func (s *Service) autoDetectOrg(ctx context.Context) (string, error) {
	orgs, err := s.client.Organizations(ctx)
	if err != nil {
		return "", fmt.Errorf("list workspaces: %w", err)
	}
	ws, err := resolve.SoleWorkspace(orgs)
	if err != nil {
		return "", err
	}
	return ws.Slug, nil
}

// resolveProvider validates an explicit --provider value, or returns the
// default ("custom") when "auto" or "" is given. The website's "Add new
// app" flow sends "custom" regardless of repo host, and we mirror that
// because it's the only value that doesn't require GitHub-App ownership
// verification or other provider-specific OAuth setup.
func resolveProvider(explicit, _ string) (string, error) {
	switch explicit {
	case "", "auto":
		return DefaultProvider, nil
	case "github", "gitlab", "bitbucket", "custom":
		return explicit, nil
	default:
		return "", fmt.Errorf("unknown provider %q (valid: auto, github, gitlab, bitbucket, custom)", explicit)
	}
}

// configIDForProjectType returns the preset config_id the server expects
// for a given project_type. Values come from
// bitrise-website/config/bitrise_ymls/custom_config.yml. Unknown
// project_types fall through to "other-config" — same fallback the server
// applies when the field is omitted.
func configIDForProjectType(projectType string) string {
	switch projectType {
	case "android":
		return "default-android-config"
	case "cordova":
		return "default-cordova-config"
	case "fastlane":
		return "default-fastlane-ios-config"
	case "flutter":
		return "flutter-config-test-ios-android-web-0"
	case "ionic":
		return "default-ionic-config"
	case "ios":
		return "default-ios-config"
	case "java":
		return "default-java-gradle-config"
	case "kotlin-multiplatform":
		return "default-kotlin-multiplatform-config"
	case "macos":
		return "default-macos-config"
	case "node-js":
		return "default-node-js-npm-config"
	case "python":
		return "default-python-pip-config"
	case "react-native":
		return "default-react-native-config"
	case "ruby":
		return "default-ruby-config"
	default:
		return "other-config"
	}
}

// deriveTitle pulls a human-readable title from a repo URL: the last path
// segment with any ".git" suffix removed. Returns "" when nothing parseable
// remains.
func deriveTitle(repoURL string) string {
	clean := strings.TrimSuffix(repoURL, ".git")
	// Strip query/fragment if present (rare for git URLs, but cheap).
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	// Try URL parse first; fall back to manual split for git@... form.
	if u, err := url.Parse(clean); err == nil && u.Path != "" {
		base := path.Base(u.Path)
		if base != "." && base != "/" {
			return base
		}
	}
	if i := strings.LastIndexAny(clean, ":/"); i >= 0 && i+1 < len(clean) {
		return clean[i+1:]
	}
	return ""
}

// GitDetector detects the cwd's git remote URL and current branch. The
// default implementation shells out to `git`. Tests inject a stub.
type GitDetector interface {
	RemoteURL(ctx context.Context) (string, error)
	CurrentBranch(ctx context.Context) (string, error)
}

// ExecGitDetector is the default GitDetector. It runs the `git` CLI in the
// current working directory.
type ExecGitDetector struct{}

// RemoteURL returns the URL of the "origin" remote, or "" if not in a git
// repo / no origin is set. Errors from git itself are returned as-is.
func (ExecGitDetector) RemoteURL(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "remote", "get-url", "origin").Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			// Not a git repo / no origin → soft failure, caller decides.
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranch returns the name of the currently-checked-out branch, or
// "" when detached or not in a git repo.
func (ExecGitDetector) CurrentBranch(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
