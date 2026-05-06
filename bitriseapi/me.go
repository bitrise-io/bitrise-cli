package bitriseapi

import "context"

// User holds profile information for the authenticated user.
type User struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// Me returns the profile of the authenticated user.
// Endpoint: GET /me
func (c *Client) Me(ctx context.Context) (*User, error) {
	user, err := get[User](ctx, c, "/me", nil)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
