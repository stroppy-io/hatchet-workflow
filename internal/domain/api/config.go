package api

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port             int
	AdminUser        string
	AdminPassword    string
	JWTSecret        string
	SettingsPath     string
	HatchetToken     string
	HatchetHost      string
	HatchetPort      int
	AllowedOrigins   []string
}

func NewConfigFromEnv() (*Config, error) {
	port := 8080
	if v := os.Getenv("STROPPY_API_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid STROPPY_API_PORT: %w", err)
		}
		port = p
	}

	hatchetPort := 7077
	if v := os.Getenv("HATCHET_CLIENT_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid HATCHET_CLIENT_PORT: %w", err)
		}
		hatchetPort = p
	}

	cfg := &Config{
		Port:           port,
		AdminUser:      os.Getenv("STROPPY_ADMIN_USER"),
		AdminPassword:  os.Getenv("STROPPY_ADMIN_PASSWORD"),
		JWTSecret:      os.Getenv("STROPPY_JWT_SECRET"),
		SettingsPath:   os.Getenv("STROPPY_SETTINGS_PATH"),
		HatchetToken:   os.Getenv("HATCHET_CLIENT_TOKEN"),
		HatchetHost:    os.Getenv("HATCHET_CLIENT_HOST"),
		HatchetPort:    hatchetPort,
	}

	if cfg.AdminUser == "" {
		return nil, fmt.Errorf("STROPPY_ADMIN_USER is required")
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("STROPPY_ADMIN_PASSWORD is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("STROPPY_JWT_SECRET is required")
	}
	if cfg.SettingsPath == "" {
		cfg.SettingsPath = "/etc/stroppy/settings.json"
	}
	if cfg.HatchetToken == "" {
		return nil, fmt.Errorf("HATCHET_CLIENT_TOKEN is required")
	}

	if v := os.Getenv("STROPPY_CORS_ORIGINS"); v != "" {
		cfg.AllowedOrigins = []string{v}
	} else {
		cfg.AllowedOrigins = []string{"*"}
	}

	return cfg, nil
}
