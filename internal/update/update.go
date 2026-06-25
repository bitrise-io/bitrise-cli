// Package update checks whether a newer bitrise-cli release is available and
// reports it to the user.
//
// The check queries the GitHub Releases API for the latest published release of
// the bitrise-io/bitrise-cli repository at most once per Interval, caching the
// result in version-check.json under the config dir so the common path makes no
// network call at all. It is best-effort by design: any failure (offline, rate
// limit, malformed response, unparseable version) yields no notice and never
// surfaces as a command error. The only network destination is GitHub's public
// API — nothing is sent to Bitrise.
//
// The cmd layer owns the policy of *when* to run a check (interactive stderr,
// human output, not CI, not opted out); this package owns the mechanics.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bitrise-io/bitrise-cli/internal/config"
)

const (
	// DefaultRepo is the GitHub "owner/repo" whose releases are checked.
	DefaultRepo = "bitrise-io/bitrise-cli"
	// DefaultAPIBaseURL is the GitHub REST API root. Overridable on Checker
	// for tests (and, in principle, GitHub Enterprise).
	DefaultAPIBaseURL = "https://api.github.com"
	// DefaultInterval is how often the network is hit. Between checks the
	// cached result is reused.
	DefaultInterval = 24 * time.Hour
	// stateFileName holds the last-check timestamp and cached latest release.
	// It sits beside config.yaml / auth.yaml in the config dir.
	stateFileName = "version-check.json"

	// maxBodyBytes caps the release-API response we read into memory.
	maxBodyBytes = 1 << 20
)

// Notice describes an available upgrade: the running version, the newer
// release, and where to get it. Versions are display strings (no leading "v").
type Notice struct {
	Current string
	Latest  string
	URL     string
}

// State is the on-disk cache. The file is read only by the CLI, so the JSON
// shape is internal.
type State struct {
	LastCheckedAt time.Time `json:"last_checked_at"`
	LatestVersion string    `json:"latest_version,omitempty"`
	ReleaseURL    string    `json:"release_url,omitempty"`
}

// Checker performs version checks. Construct one with New for normal use; tests
// build it directly and override fields. A zero field is filled with its
// default by Check, except StatePath: an empty StatePath disables caching (no
// file is read or written), which forces every Check to hit the network.
type Checker struct {
	Current    string       // the running CLI version
	Repo       string       // GitHub "owner/repo"
	APIBaseURL string       // GitHub REST API root
	StatePath  string       // cache file path; empty disables caching
	HTTP       *http.Client // HTTP client for the release fetch
	Now        func() time.Time
	Interval   time.Duration
}

// New returns a Checker wired to the production GitHub repo, with its cache
// stored next to config.yaml in the config dir.
func New(current string) (*Checker, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	return &Checker{
		Current:    current,
		Repo:       DefaultRepo,
		APIBaseURL: DefaultAPIBaseURL,
		StatePath:  filepath.Join(dir, stateFileName),
		// A self-contained timeout in case a caller forgets a context deadline;
		// the cmd layer also bounds the call with its own (shorter) context.
		HTTP:     &http.Client{Timeout: 5 * time.Second},
		Now:      time.Now,
		Interval: DefaultInterval,
	}, nil
}

func (c *Checker) applyDefaults() {
	if c.Repo == "" {
		c.Repo = DefaultRepo
	}
	if c.APIBaseURL == "" {
		c.APIBaseURL = DefaultAPIBaseURL
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 5 * time.Second}
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Interval <= 0 {
		c.Interval = DefaultInterval
	}
}

// Check returns a Notice when a newer release than Current exists, or nil
// otherwise. When the cached result is fresh (younger than Interval) it makes
// no network call. On a refresh it records the attempt — success or failure —
// so a transient outage backs off for Interval rather than re-fetching on
// every command. A non-nil error reports the most recent fetch failure for
// observability; callers may safely ignore it, since a usable cached result
// still produces a Notice and an unusable one produces nil.
func (c *Checker) Check(ctx context.Context) (*Notice, error) {
	c.applyDefaults()

	st, _ := loadState(c.StatePath) // a missing/corrupt cache is treated as empty

	var fetchErr error
	if c.due(st.LastCheckedAt) {
		latest, url, err := c.fetchLatest(ctx)
		st.LastCheckedAt = c.Now().UTC()
		if err != nil {
			fetchErr = err
		} else if latest != "" {
			st.LatestVersion, st.ReleaseURL = latest, url
		}
		_ = saveState(c.StatePath, st) // best-effort; next run just re-checks
	}

	if st.LatestVersion != "" && isNewer(c.Current, st.LatestVersion) {
		return &Notice{
			Current: displayVersion(c.Current),
			Latest:  displayVersion(st.LatestVersion),
			URL:     st.ReleaseURL,
		}, fetchErr
	}
	return nil, fetchErr
}

// due reports whether enough time has passed since the last check (or one has
// never run) to warrant hitting the network.
func (c *Checker) due(last time.Time) bool {
	return last.IsZero() || c.Now().Sub(last) >= c.Interval
}

// release is the subset of GitHub's release JSON we consume. The
// /releases/latest endpoint already excludes drafts and pre-releases, so the
// tag it returns is the newest stable release.
type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func (c *Checker) fetchLatest(ctx context.Context) (version, url string, err error) {
	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", c.APIBaseURL, c.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", fmt.Errorf("build request: %w", err)
	}
	// GitHub rejects requests without a User-Agent; the recommended media type
	// pins the response schema.
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bitrise-cli/"+c.Current)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("github releases API: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}
	var rel release
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	return rel.TagName, rel.HTMLURL, nil
}

// loadState reads the cache file. A missing file (first run) or a corrupt one
// is reported as an empty State with no error, so a bad cache just triggers a
// fresh check instead of breaking the command.
func loadState(path string) (State, error) {
	if path == "" {
		return State{}, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from the config dir, not user input
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read %s: %w", path, err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, nil // corrupt cache → treat as empty
	}
	return st, nil
}

// saveState atomically writes the cache with 0600 perms, creating the parent
// dir (0700) if needed — matching config.yaml / auth.yaml. An empty path
// disables caching and is a no-op.
func saveState(path string, st State) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal version-check state: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}
