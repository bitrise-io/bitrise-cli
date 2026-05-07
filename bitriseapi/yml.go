package bitriseapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// AppBitriseYML fetches the stored bitrise.yml for an app.
// Endpoint: GET /apps/{app-slug}/bitrise.yml
// The API returns plain text YAML.
func (c *Client) AppBitriseYML(ctx context.Context, appSlug string) (string, error) {
	req, err := c.newRequest(ctx, "/apps/"+appSlug+"/bitrise.yml", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")
	body, err := c.do(req)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// BuildBitriseYML fetches the bitrise.yml that a specific build ran with.
// Endpoint: GET /apps/{app-slug}/builds/{build-slug}/bitrise.yml
// The API returns plain text YAML.
func (c *Client) BuildBitriseYML(ctx context.Context, appSlug, buildSlug string) (string, error) {
	path := "/apps/" + appSlug + "/builds/" + buildSlug + "/bitrise.yml"
	req, err := c.newRequest(ctx, path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")
	body, err := c.do(req)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// AppConfigUpdateRequest is the body for POST /apps/{app-slug}/bitrise.yml.
// The yaml field must be the parsed YAML content (a Go map/slice structure),
// not a raw YAML string, because the API expects a JSON object.
type AppConfigUpdateRequest struct {
	AppConfigDatastoreYAML any `json:"app_config_datastore_yaml"`
}

// UpdateAppBitriseYML uploads a new bitrise.yml for an app.
// Endpoint: POST /apps/{app-slug}/bitrise.yml
// content must be the YAML already parsed into a Go value (e.g. map[string]any).
func (c *Client) UpdateAppBitriseYML(ctx context.Context, appSlug string, content any) error {
	req := AppConfigUpdateRequest{AppConfigDatastoreYAML: content}
	_, err := postDecode[AppConfigUpdateRequest, map[string]any](ctx, c, "/apps/"+appSlug+"/bitrise.yml", req)
	return err
}

// ValidateBitriseYMLRequest is the JSON body for POST /validate-bitrise-yml.
type ValidateBitriseYMLRequest struct {
	BitriseYML string `json:"bitrise_yml"`
}

// ValidateBitriseYMLResponse is the 200 response from POST /validate-bitrise-yml.
type ValidateBitriseYMLResponse struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// ValidateBitriseYML validates a bitrise.yml string via the API.
// Endpoint: POST /validate-bitrise-yml
// appSlug is optional; when non-empty it enables app-specific validation
// (stack IDs, machine types, license pools).
func (c *Client) ValidateBitriseYML(ctx context.Context, yamlContent, appSlug string) (ValidateBitriseYMLResponse, error) {
	var zero ValidateBitriseYMLResponse

	data, err := json.Marshal(ValidateBitriseYMLRequest{BitriseYML: yamlContent})
	if err != nil {
		return zero, fmt.Errorf("marshal request: %w", err)
	}

	u, err := url.Parse(c.baseURL + "/validate-bitrise-yml")
	if err != nil {
		return zero, fmt.Errorf("parse URL: %w", err)
	}
	if appSlug != "" {
		u.RawQuery = url.Values{"app_slug": {appSlug}}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(data))
	if err != nil {
		return zero, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	body, err := c.do(req)
	if err != nil {
		return zero, err
	}

	var resp ValidateBitriseYMLResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}
