package bitriseapi

import "context"

// RegisterAppRequest is the body of POST /apps/register.
//
// IsPublic is a non-pointer bool with no omitempty so the JSON always
// carries `is_public:false` when not opted in — matching the website's
// "Add new app" flow which sends the field explicitly. FlowType is the
// analytics attribution string ("cli" / "website") that the server reads
// out of params[:flow_type].
type RegisterAppRequest struct {
	RepoURL           string `json:"repo_url"`
	OrganizationSlug  string `json:"organization_slug"`
	Provider          string `json:"provider"`
	IsPublic          bool   `json:"is_public"`
	Title             string `json:"title,omitempty"`
	DefaultBranchName string `json:"default_branch_name,omitempty"`
	FlowType          string `json:"flow_type,omitempty"`
}

// RegisterAppResponse is the response from POST /apps/register.
type RegisterAppResponse struct {
	Status string `json:"status"`
	Slug   string `json:"slug"`
}

// RegisterApp creates a new app on Bitrise. The returned app is in
// status=-1 (setup-incomplete) until FinishApp is called.
// Endpoint: POST /apps/register.
func (c *Client) RegisterApp(ctx context.Context, req RegisterAppRequest) (RegisterAppResponse, error) {
	return postDecode[RegisterAppRequest, RegisterAppResponse](ctx, c, "/apps/register", req)
}

// FinishAppRequest is the body of POST /apps/{slug}/finish.
type FinishAppRequest struct {
	StackID     string            `json:"stack_id"`
	Mode        string            `json:"mode"`
	ProjectType string            `json:"project_type,omitempty"`
	Config      string            `json:"config,omitempty"`
	Envs        map[string]string `json:"envs,omitempty"`
	FlowType    string            `json:"flow_type,omitempty"`
}

// FinishAppResponse is the response from POST /apps/{slug}/finish.
type FinishAppResponse struct {
	Status                    string `json:"status"`
	BuildTriggerToken         string `json:"build_trigger_token"`
	BranchName                string `json:"branch_name"`
	IsWebhookAutoRegSupported bool   `json:"is_webhook_auto_reg_supported"`
}

// FinishApp activates a registered app, returning its build trigger token.
// Endpoint: POST /apps/{slug}/finish.
func (c *Client) FinishApp(ctx context.Context, appSlug string, req FinishAppRequest) (FinishAppResponse, error) {
	return postDecode[FinishAppRequest, FinishAppResponse](ctx, c, "/apps/"+appSlug+"/finish", req)
}

// uploadAppConfigRequest is the body of POST /apps/{slug}/bitrise.yml.
type uploadAppConfigRequest struct {
	AppConfigDatastoreYAML string `json:"app_config_datastore_yaml"`
}

// uploadAppConfigResponse is the response from POST /apps/{slug}/bitrise.yml.
type uploadAppConfigResponse struct {
	Status string `json:"status"`
}

// UploadAppConfig uploads the given YAML as the app's bitrise.yml.
// Endpoint: POST /apps/{slug}/bitrise.yml.
func (c *Client) UploadAppConfig(ctx context.Context, appSlug string, yaml string) error {
	_, err := postDecode[uploadAppConfigRequest, uploadAppConfigResponse](
		ctx, c, "/apps/"+appSlug+"/bitrise.yml",
		uploadAppConfigRequest{AppConfigDatastoreYAML: yaml},
	)
	return err
}
