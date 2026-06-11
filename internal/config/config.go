// Package config persists and reads the user's CLI defaults.
//
// Storage: YAML at $XDG_CONFIG_HOME/bitrise/config.yaml, falling back to
// ~/.config/bitrise/config.yaml. Written with 0600 permissions.
//
// The package is the single source of truth for: known config keys, env
// var names, default values, and validation rules. The cmd layer composes
// config values via Resolve and reads the result from cmd.Context().
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
)

// Known config keys. These names are part of the user-facing CLI
// contract — `bitrise-cli config set <key> <value>` references them directly.
const (
	KeyOutput             = "output"
	KeyAppID              = "app_id"
	KeyDefaultWorkspaceID = "default_workspace_id"
	KeyAPIBaseURL         = "api_base_url"
	KeyRDEAPIBaseURL      = "rde_api_base_url"
	KeyWebBaseURL         = "web_base_url"
	KeyTheme              = "theme"
)

// Keys is the registered list of config keys, used for validation and help.
var Keys = []string{KeyOutput, KeyAppID, KeyDefaultWorkspaceID, KeyAPIBaseURL, KeyRDEAPIBaseURL, KeyWebBaseURL, KeyTheme}

// Config is the on-disk shape. Fields use omitempty so unset values
// don't appear in the saved YAML.
type Config struct {
	Output             string `yaml:"output,omitempty"`
	AppID              string `yaml:"app_id,omitempty"`
	DefaultWorkspaceID string `yaml:"default_workspace_id,omitempty"`
	APIBaseURL         string `yaml:"api_base_url,omitempty"`
	RDEAPIBaseURL      string `yaml:"rde_api_base_url,omitempty"`
	WebBaseURL         string `yaml:"web_base_url,omitempty"`
	Theme              string `yaml:"theme,omitempty"`
}

// UnmarshalYAML reads a Config, accepting the legacy key names `app_slug` and
// `default_workspace_slug` as fallbacks for the current `app_id` and
// `default_workspace_id`. This keeps config files written by older versions
// readable (so a saved default app/workspace isn't silently lost on upgrade);
// the next Save rewrites the file with the current key names. The current key
// wins if a file somehow carries both.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	// NOTE: new Config fields must be added here too (two places: the struct and the copy block below).
	var raw struct {
		Output               string `yaml:"output"`
		AppID                string `yaml:"app_id"`
		AppSlugLegacy        string `yaml:"app_slug"`
		DefaultWorkspaceID   string `yaml:"default_workspace_id"`
		DefaultWorkspaceSlug string `yaml:"default_workspace_slug"`
		APIBaseURL           string `yaml:"api_base_url"`
		RDEAPIBaseURL        string `yaml:"rde_api_base_url"`
		WebBaseURL           string `yaml:"web_base_url"`
		Theme                string `yaml:"theme"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	// NOTE: new Config fields must be added here too (and to the raw struct above).
	c.Output = raw.Output
	c.AppID = firstNonEmpty(raw.AppID, raw.AppSlugLegacy)
	c.DefaultWorkspaceID = firstNonEmpty(raw.DefaultWorkspaceID, raw.DefaultWorkspaceSlug)
	c.APIBaseURL = raw.APIBaseURL
	c.RDEAPIBaseURL = raw.RDEAPIBaseURL
	c.WebBaseURL = raw.WebBaseURL
	c.Theme = raw.Theme
	return nil
}

// UnmarshalYAML reads a Config, accepting the legacy key names `app_slug` and
// `default_workspace_slug` as fallbacks for the current `app_id` and
// `default_workspace_id`. This keeps config files written by older versions
// readable (so a saved default app/workspace isn't silently lost on upgrade);
// the next Save rewrites the file with the current key names. The current key
// wins if a file somehow carries both.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Output               string `yaml:"output"`
		AppID                string `yaml:"app_id"`
		AppSlugLegacy        string `yaml:"app_slug"`
		DefaultWorkspaceID   string `yaml:"default_workspace_id"`
		DefaultWorkspaceSlug string `yaml:"default_workspace_slug"`
		APIBaseURL           string `yaml:"api_base_url"`
		RDEAPIBaseURL        string `yaml:"rde_api_base_url"`
		WebBaseURL           string `yaml:"web_base_url"`
		Theme                string `yaml:"theme"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	c.Output = raw.Output
	c.AppID = firstNonEmpty(raw.AppID, raw.AppSlugLegacy)
	c.DefaultWorkspaceID = firstNonEmpty(raw.DefaultWorkspaceID, raw.DefaultWorkspaceSlug)
	c.APIBaseURL = raw.APIBaseURL
	c.RDEAPIBaseURL = raw.RDEAPIBaseURL
	c.WebBaseURL = raw.WebBaseURL
	c.Theme = raw.Theme
	return nil
}

// DirFileName is the file looked up in the working directory and its
// ancestors to provide per-project config. Per the patterns guide, this is
// the third-highest-precedence layer (above the global file, below env vars
// and CLI flags).
const DirFileName = ".bitrise-cli.yml"

// Path returns the absolute path to the global config file (whether or not
// it exists). Honors XDG_CONFIG_HOME, falling back to ~/.config.
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate user home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bitrise", "config.yaml"), nil
}

// Load reads and validates the config file. A missing file is not an error —
// it returns the zero Config so first-time users don't see failures.
func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(p) //nolint:gosec // p is derived from XDG_CONFIG_HOME / user home, not user input
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", p, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", p, err)
	}
	if err := c.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid %s: %w", p, err)
	}
	return c, nil
}

// LoadDir searches the current working directory and its ancestors for a
// per-project config file (DirFileName). Returns the parsed config, the
// absolute path of the file that was used (empty if none found), and any
// parse/validation error. A missing file at all levels is not an error.
func LoadDir() (Config, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, "", fmt.Errorf("get working dir: %w", err)
	}
	return loadDirFrom(cwd)
}

func loadDirFrom(start string) (Config, string, error) {
	for dir := start; ; {
		p := filepath.Join(dir, DirFileName)
		data, err := os.ReadFile(p) //nolint:gosec // p is an ancestor-directory config path, not user input
		if err == nil {
			var c Config
			if err := yaml.Unmarshal(data, &c); err != nil {
				return Config{}, "", fmt.Errorf("parse %s: %w", p, err)
			}
			if err := c.Validate(); err != nil {
				return Config{}, "", fmt.Errorf("invalid %s: %w", p, err)
			}
			return c, p, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return Config{}, "", fmt.Errorf("read %s: %w", p, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Config{}, "", nil // reached filesystem root
		}
		dir = parent
	}
}

// Save validates and atomically writes c to disk with 0600 permissions.
// It creates the parent directory (0700) if missing.
func Save(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(&c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("install %s: %w", p, err)
	}
	return nil
}

// Validate reports the first invalid field, if any.
func (c *Config) Validate() error {
	if c.Output != "" {
		if _, err := output.ParseFormat(c.Output); err != nil {
			return fmt.Errorf("field %q: %w", KeyOutput, err)
		}
	}
	if c.APIBaseURL != "" {
		u, err := url.Parse(c.APIBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("field %q: not a valid URL: %s", KeyAPIBaseURL, c.APIBaseURL)
		}
	}
	if c.RDEAPIBaseURL != "" {
		u, err := url.Parse(c.RDEAPIBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("field %q: not a valid URL: %s", KeyRDEAPIBaseURL, c.RDEAPIBaseURL)
		}
	}
	if c.WebBaseURL != "" {
		u, err := url.Parse(c.WebBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("field %q: not a valid URL: %s", KeyWebBaseURL, c.WebBaseURL)
		}
	}
	if c.Theme != "" {
		if _, err := style.ParseTheme(c.Theme); err != nil {
			return fmt.Errorf("field %q: %w", KeyTheme, err)
		}
	}
	return nil
}

// Get returns the stored value of a known key.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case KeyOutput:
		return c.Output, nil
	case KeyAppID:
		return c.AppID, nil
	case KeyDefaultWorkspaceID:
		return c.DefaultWorkspaceID, nil
	case KeyAPIBaseURL:
		return c.APIBaseURL, nil
	case KeyRDEAPIBaseURL:
		return c.RDEAPIBaseURL, nil
	case KeyWebBaseURL:
		return c.WebBaseURL, nil
	case KeyTheme:
		return c.Theme, nil
	default:
		return "", unknownKeyErr(key)
	}
}

// Set assigns value to key and validates the resulting config. If validation
// fails, c is left untouched — callers can rely on Set being all-or-nothing.
func (c *Config) Set(key, value string) error {
	next := *c
	switch key {
	case KeyOutput:
		next.Output = value
	case KeyAppID:
		next.AppID = value
	case KeyDefaultWorkspaceID:
		next.DefaultWorkspaceID = value
	case KeyAPIBaseURL:
		next.APIBaseURL = value
	case KeyRDEAPIBaseURL:
		next.RDEAPIBaseURL = value
	case KeyWebBaseURL:
		next.WebBaseURL = value
	case KeyTheme:
		next.Theme = value
	default:
		return unknownKeyErr(key)
	}
	if err := next.Validate(); err != nil {
		return err
	}
	*c = next
	return nil
}

// Unset clears the value of key (equivalent to Set with empty string).
func (c *Config) Unset(key string) error {
	return c.Set(key, "")
}

func unknownKeyErr(key string) error {
	return fmt.Errorf("unknown config key %q (valid keys: %s)", key, strings.Join(Keys, ", "))
}
