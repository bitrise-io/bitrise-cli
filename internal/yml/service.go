// Package yml holds the business-logic layer for bitrise.yml operations.
package yml

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

// GetResult holds the retrieved bitrise.yml content.
type GetResult struct {
	AppSlug   string `json:"app_id"`
	BuildSlug string `json:"build_id,omitempty"`
	Content   string `json:"content"`
}

// ValidateResult holds the outcome of a bitrise.yml validation.
type ValidateResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// Service exposes bitrise.yml operations to the cmd layer.
type Service struct {
	client *bitriseapi.Client
}

// NewService returns a Service backed by the given API client.
func NewService(client *bitriseapi.Client) *Service {
	return &Service{client: client}
}

// Get retrieves the stored bitrise.yml for an app.
// When buildSlug is non-empty, it retrieves the yml used for that specific build instead.
func (s *Service) Get(ctx context.Context, appSlug, buildSlug string) (GetResult, error) {
	if s.client == nil {
		return GetResult{}, fmt.Errorf("API client not configured")
	}
	if appSlug == "" {
		return GetResult{}, fmt.Errorf("app ID is required")
	}
	var (
		content string
		err     error
	)
	if buildSlug != "" {
		content, err = s.client.BuildBitriseYML(ctx, appSlug, buildSlug)
	} else {
		content, err = s.client.AppBitriseYML(ctx, appSlug)
	}
	if err != nil {
		if apiErr, ok := errors.AsType[*bitriseapi.APIError](err); ok && apiErr.StatusCode == http.StatusNotFound {
			if buildSlug != "" {
				return GetResult{}, fmt.Errorf("build %q not found", buildSlug)
			}
			return GetResult{}, fmt.Errorf("app %q not found", appSlug)
		}
		return GetResult{}, err
	}
	return GetResult{AppSlug: appSlug, BuildSlug: buildSlug, Content: content}, nil
}

// Update uploads rawYAML as the new stored bitrise.yml for the given app.
// The YAML is parsed before sending so the API receives a JSON object.
func (s *Service) Update(ctx context.Context, appSlug, rawYAML string) error {
	if s.client == nil {
		return fmt.Errorf("API client not configured")
	}
	if appSlug == "" {
		return fmt.Errorf("app ID is required")
	}
	var parsed any
	if err := yaml.Unmarshal([]byte(rawYAML), &parsed); err != nil {
		return fmt.Errorf("parse bitrise.yml: %w", err)
	}
	if err := s.client.UpdateAppBitriseYML(ctx, appSlug, parsed); err != nil {
		if apiErr, ok := errors.AsType[*bitriseapi.APIError](err); ok && apiErr.StatusCode == http.StatusNotFound {
			return fmt.Errorf("app %q not found", appSlug)
		}
		return err
	}
	return nil
}

// Validate sends rawYAML to the API validation endpoint.
// appSlug is optional; when provided the validation is performed against
// app-specific settings (stacks, machine types, license pools).
//
// A 422 response (unparseable YAML) is normalized into a ValidateResult
// with Valid=false rather than propagated as an error.
func (s *Service) Validate(ctx context.Context, rawYAML, appSlug string) (ValidateResult, error) {
	if s.client == nil {
		return ValidateResult{}, fmt.Errorf("API client not configured")
	}
	resp, err := s.client.ValidateBitriseYML(ctx, rawYAML, appSlug)
	if err != nil {
		var apiErr *bitriseapi.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 422 {
			return ValidateResult{Valid: false, Errors: []string{apiErr.Message}}, nil
		}
		return ValidateResult{}, err
	}
	errs := resp.Errors
	if errs == nil {
		errs = []string{}
	}
	warns := resp.Warnings
	if warns == nil {
		warns = []string{}
	}
	return ValidateResult{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: warns,
	}, nil
}
