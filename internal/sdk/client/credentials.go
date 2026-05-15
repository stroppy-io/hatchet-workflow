package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Credentials holds the server address and auth tokens for a single server.
type Credentials struct {
	Server       string    `json:"server"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Context holds the currently active server and tenant.
type Context struct {
	CurrentServer string `json:"current_server"`
	CurrentTenant string `json:"current_tenant_id"`
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "stroppy-cloud")
	if err := os.MkdirAll(out, 0o700); err != nil {
		return "", err
	}
	return out, nil
}

// LoadCredentials reads persisted credentials for the given server.
func LoadCredentials(server string) (*Credentials, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		return nil, err
	}
	var all map[string]*Credentials
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	return all[server], nil
}

// SaveCredentials writes credentials for c.Server to the credentials file.
func SaveCredentials(c *Credentials) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "credentials.json")

	all := map[string]*Credentials{}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &all)
	}
	all[c.Server] = c
	out, _ := json.MarshalIndent(all, "", "  ")
	return os.WriteFile(path, out, 0o600)
}

// LoadContext reads the active context from disk.
func LoadContext() (*Context, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "context.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &Context{}, nil
		}
		return nil, err
	}
	c := &Context{}
	return c, json.Unmarshal(raw, c)
}

// SaveContext writes the active context to disk.
func SaveContext(c *Context) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(filepath.Join(dir, "context.json"), out, 0o600)
}
