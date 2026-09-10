// Package iam is the hand-written slice of the IAM HTTP API the server
// needs beyond token verification: the SDK exposes no typed client for
// these calls, so a small net/http client covers them.
package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Users reads the runtime user API on behalf of the end user.
type Users struct {
	base     string
	clientID string
	env      string
	hc       *http.Client
}

// NewUsers builds the client.
func NewUsers(baseURL, clientID, environment string) *Users {
	return &Users{
		base: strings.TrimRight(baseURL, "/"), clientID: clientID, env: environment,
		hc: &http.Client{Timeout: 5 * time.Second},
	}
}

// User is what the profile mirrors.
type User struct {
	ID            string `json:"id"`
	Email         string `json:"primary_email"`
	EmailVerified bool   `json:"email_verified"`
	Profile       struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	} `json:"profile"`
}

// Me is GET /v1/users/me with the user's own access token — the only way
// to learn the email, which the access token deliberately omits.
func (u *Users) Me(ctx context.Context, accessToken string) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.base+"/v1/users/me", http.NoBody)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Client-Id", u.clientID)
	if u.env != "" {
		req.Header.Set("X-Environment", u.env)
	}
	resp, err := u.hc.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("iam users/me: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("iam users/me: status %d", resp.StatusCode)
	}
	var body struct {
		User json.RawMessage `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return User{}, fmt.Errorf("iam users/me: decode: %w", err)
	}
	var user User
	if err := json.Unmarshal(body.User, &user); err != nil {
		return User{}, fmt.Errorf("iam users/me: decode user: %w", err)
	}
	return user, nil
}
