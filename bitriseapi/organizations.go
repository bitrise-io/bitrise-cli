package bitriseapi

import "context"

// Organization is a workspace the authenticated user belongs to. Field
// names match v0.OrganizationResponseModel in the Bitrise API swagger.
type Organization struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Organizations returns the organizations (workspaces) the authenticated
// user can access. Endpoint: GET /organizations.
func (c *Client) Organizations(ctx context.Context) ([]Organization, error) {
	page, err := getPage[Organization](ctx, c, "/organizations", nil)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}
