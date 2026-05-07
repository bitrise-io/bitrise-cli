package user

import (
	"context"

	"github.com/bitrise-io/bitrise-cli/bitriseapi"
)

// Profile is the CLI-facing representation of the authenticated user.
type Profile struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ProfileService exposes user profile operations backed by an API token.
type ProfileService struct {
	client *bitriseapi.Client
}

// NewProfileService returns a ProfileService backed by the given API client.
func NewProfileService(client *bitriseapi.Client) *ProfileService {
	return &ProfileService{client: client}
}

// Me returns the profile of the authenticated user.
func (s *ProfileService) Me(ctx context.Context) (Profile, error) {
	u, err := s.client.Me(ctx)
	if err != nil {
		return Profile{}, err
	}
	return Profile{Username: u.Username, Email: u.Email, AvatarURL: u.AvatarURL}, nil
}
