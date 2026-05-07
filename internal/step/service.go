// Package step holds the business-logic layer for step operations.
package step

import (
	"context"
	"fmt"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

// StepInput describes a single step input or output.
type StepInput struct {
	Name         string   `json:"name"`
	Title        string   `json:"title,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Description  string   `json:"description,omitempty"`
	DefaultValue string   `json:"default_value,omitempty"`
	IsRequired   bool     `json:"is_required,omitempty"`
	IsSensitive  bool     `json:"is_sensitive,omitempty"`
	ValueOptions []string `json:"value_options,omitempty"`
}

// Step is the CLI representation of a step search result.
type Step struct {
	ID                  string      `json:"id"`
	StepRef             string      `json:"step_ref"`
	Title               string      `json:"title"`
	Summary             string      `json:"summary,omitempty"`
	Version             string      `json:"version,omitempty"`
	LatestVersionNumber string      `json:"latest_version_number,omitempty"`
	Maintainer          string      `json:"maintainer,omitempty"`
	IsDeprecated        bool        `json:"is_deprecated,omitempty"`
	Inputs              []StepInput `json:"inputs,omitempty"`
}

// SearchResult holds step search results.
type SearchResult struct {
	Items []Step `json:"items"`
}

// SearchOptions filters a step search.
type SearchOptions struct {
	Query       string
	Categories  []string
	Maintainers []string
}

// InputsResult holds the inputs for a step version.
type InputsResult struct {
	StepRef string      `json:"step_ref"`
	Items   []StepInput `json:"items"`
}

// Service exposes step operations to the cmd layer.
type Service struct {
	client *bitriseapi.Client
}

// NewService returns a Service backed by the given API client.
func NewService(client *bitriseapi.Client) *Service {
	return &Service{client: client}
}

// Search returns steps matching query and filters.
func (s *Service) Search(ctx context.Context, opts SearchOptions) (SearchResult, error) {
	if s.client == nil {
		return SearchResult{}, fmt.Errorf("API client not configured")
	}
	raw, err := s.client.SearchSteps(ctx, bitriseapi.StepSearchOptions{
		Query:       opts.Query,
		Categories:  opts.Categories,
		Maintainers: opts.Maintainers,
	})
	if err != nil {
		return SearchResult{}, err
	}
	items := make([]Step, 0, len(raw))
	for _, r := range raw {
		items = append(items, stepFromAPI(r))
	}
	return SearchResult{Items: items}, nil
}

// Inputs returns the inputs for the given step reference.
func (s *Service) Inputs(ctx context.Context, stepRef string) (InputsResult, error) {
	if s.client == nil {
		return InputsResult{}, fmt.Errorf("API client not configured")
	}
	if stepRef == "" {
		return InputsResult{}, fmt.Errorf("step ref is required")
	}
	raw, err := s.client.StepInputs(ctx, stepRef)
	if err != nil {
		return InputsResult{}, err
	}
	items := make([]StepInput, 0, len(raw))
	for _, r := range raw {
		items = append(items, inputFromAPI(r))
	}
	return InputsResult{StepRef: stepRef, Items: items}, nil
}

func stepFromAPI(r bitriseapi.StepResponse) Step {
	inputs := make([]StepInput, 0, len(r.Inputs))
	for _, inp := range r.Inputs {
		inputs = append(inputs, inputFromAPI(inp))
	}
	return Step{
		ID:                  r.ID,
		StepRef:             r.StepRef,
		Title:               r.Title,
		Summary:             r.Summary,
		Version:             r.Version,
		LatestVersionNumber: r.LatestVersionNumber,
		Maintainer:          r.Maintainer,
		IsDeprecated:        r.IsDeprecated,
		Inputs:              inputs,
	}
}

func inputFromAPI(r bitriseapi.StepInputOutputResponse) StepInput {
	return StepInput{
		Name:         r.Name,
		Title:        r.Title,
		Summary:      r.Summary,
		Description:  r.Description,
		DefaultValue: r.DefaultValue,
		IsRequired:   r.IsRequired,
		IsSensitive:  r.IsSensitive,
		ValueOptions: r.ValueOptions,
	}
}
