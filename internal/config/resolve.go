package config

import (
	"context"
	"os"

	"github.com/bitrise-io/bitrise-cli/internal/output"
)

// Environment variables that override config-file values.
const (
	EnvToken      = "BITRISE_TOKEN"
	EnvAppSlug    = "BITRISE_APP_SLUG"
	EnvOutput     = "BITRISE_OUTPUT"
	EnvAPIBaseURL = "BITRISE_API_BASE_URL"
)

// DefaultAPIBaseURL is the production Bitrise API base URL.
const DefaultAPIBaseURL = "https://api.bitrise.io/v0.1"

// Resolved is the merged settings the cmd layer reads on every invocation.
//
// Layering per the CLI patterns guide, highest to lowest precedence:
//  1. CLI flag (only the persistent --output flag is folded in here;
//     per-command flags like --app are layered in the command handlers)
//  2. Environment variables
//  3. Per-directory config (.bitrise-cli.yml in CWD or ancestors)
//  4. Global config file (~/.config/bitrise/config.yaml)
//  5. Built-in defaults
type Resolved struct {
	Output     output.Format
	AppSlug    string
	Token      string
	APIBaseURL string
}

// Resolve merges global and per-directory configs with environment variables
// and the persistent --output flag value. flagOutput may be empty when unset.
// dirCfg is the zero Config when no per-directory file was found.
func Resolve(globalCfg, dirCfg Config, flagOutput string) (Resolved, error) {
	var r Resolved

	rawOutput := flagOutput
	if rawOutput == "" {
		rawOutput = firstNonEmpty(os.Getenv(EnvOutput), dirCfg.Output, globalCfg.Output)
	}
	f, err := output.ParseFormat(rawOutput)
	if err != nil {
		return Resolved{}, err
	}
	r.Output = f

	r.AppSlug = firstNonEmpty(os.Getenv(EnvAppSlug), dirCfg.AppSlug, globalCfg.AppSlug)
	r.Token = firstNonEmpty(os.Getenv(EnvToken), dirCfg.Token, globalCfg.Token)
	r.APIBaseURL = firstNonEmpty(os.Getenv(EnvAPIBaseURL), dirCfg.APIBaseURL, globalCfg.APIBaseURL, DefaultAPIBaseURL)

	return r, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

type ctxKey struct{}

// WithResolved stores r on ctx so command handlers can read it.
func WithResolved(ctx context.Context, r Resolved) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext retrieves Resolved from ctx, or a zero value if absent.
// The zero value's Output ("") will fail at format dispatch — callers
// should never see it in practice because root's PersistentPreRunE always
// installs a Resolved before any subcommand runs.
func FromContext(ctx context.Context) Resolved {
	if r, ok := ctx.Value(ctxKey{}).(Resolved); ok {
		return r
	}
	return Resolved{}
}
