package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Profile struct {
	Server       string `json:"server"`
	Tenant       string `json:"tenant,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type Credentials struct {
	Profiles map[string]*Profile `json:"profiles"`
	Current  string              `json:"current"`
}

func credentialsPath() string {
	if dir := os.Getenv("STROPPY_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "credentials.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "stroppy-cloud", "credentials.json")
}

func loadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{Profiles: map[string]*Profile{}, Current: "default"}, nil
		}
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	return &c, nil
}

func (c *Credentials) Save() error {
	path := credentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (c *Credentials) CurrentProfile() *Profile {
	if p, ok := c.Profiles[c.Current]; ok {
		return p
	}
	return nil
}
