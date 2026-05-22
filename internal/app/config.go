// Package app assembles the stroppy-cloud server (grpc API + dag processor + agent
// queue) and the agent loop from configuration. It is the single composition root
// shared by the `serve` and `agent` subcommands of cmd/stroppy-cloud.
package app

import (
	"os"
	"strconv"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
)

// Config is the composition-root configuration, loaded from the environment. It
// also satisfies the per-service config interfaces (auth.Config, share.Config,
// deploy.Config) so the same struct is threaded through the whole assembly.
type Config struct {
	// GRPCAddr is the listen address for the grpc API (server mode).
	GRPCAddr string

	JWTSecretRaw  string
	ShareBaseURL_ string

	Postgres postgres.Config
	Valkey   valkey.Config
	S3       s3.Config

	VictoriaLogsURL    string
	VictoriaMetricsURL string
	VictoriaToken      string

	// Deployment seam: where provisioned agents call back + fetch their binary.
	ServerAddr_     string
	AgentBinaryURL_ string
	AgentJWTTTL_    time.Duration

	// Agent client mode (the `agent` subcommand).
	Agent AgentConfig
}

// AgentConfig configures the agent poll loop.
type AgentConfig struct {
	ServerAddr string // grpc address of the control plane
	TenantID   string
	MachineID  string
	Token      string // agent JWT (Bearer)
	PollEvery  time.Duration
}

// ── per-service config interface satisfaction ──────────────────────────────────

func (c *Config) JWTSecret() []byte              { return []byte(c.JWTSecretRaw) }
func (c *Config) ShareBaseURL() string           { return c.ShareBaseURL_ }
func (c *Config) ServerAddr() string             { return c.ServerAddr_ }
func (c *Config) AgentBinaryURL() string         { return c.AgentBinaryURL_ }
func (c *Config) AccessTokenTTL() time.Duration  { return 15 * time.Minute }
func (c *Config) RefreshTokenTTL() time.Duration { return 30 * 24 * time.Hour }
func (c *Config) AgentJWTTTL() time.Duration {
	if c.AgentJWTTTL_ <= 0 {
		return 24 * time.Hour
	}
	return c.AgentJWTTTL_
}

// LoadConfig reads configuration from the environment with sane local defaults.
func LoadConfig() *Config {
	c := &Config{
		GRPCAddr:      env("STROPPY_GRPC_ADDR", ":8080"),
		JWTSecretRaw:  env("STROPPY_JWT_SECRET", "stroppy-dev-secret-change-me"),
		ShareBaseURL_: env("STROPPY_SHARE_BASE_URL", "http://localhost:8080/share/"),
		Postgres: postgres.Config{
			Host:     env("STROPPY_PG_HOST", "localhost"),
			Port:     envInt("STROPPY_PG_PORT", 5432),
			Username: env("STROPPY_PG_USER", "stroppy"),
			Password: env("STROPPY_PG_PASSWORD", "stroppy"),
			Database: env("STROPPY_PG_DATABASE", "stroppy"),
		},
		Valkey: valkey.Config{
			Addresses: []string{env("STROPPY_VALKEY_ADDR", "localhost:6379")},
			Username:  os.Getenv("STROPPY_VALKEY_USER"),
			Password:  os.Getenv("STROPPY_VALKEY_PASSWORD"),
		},
		S3: s3.Config{
			Endpoint:  os.Getenv("STROPPY_S3_ENDPOINT"),
			Region:    env("STROPPY_S3_REGION", "us-east-1"),
			AccessKey: os.Getenv("STROPPY_S3_ACCESS_KEY"),
			SecretKey: os.Getenv("STROPPY_S3_SECRET_KEY"),
			Bucket:    env("STROPPY_S3_BUCKET", "stroppy-packages"),
			UseSSL:    envBool("STROPPY_S3_USE_SSL", false),
		},
		VictoriaLogsURL:    env("STROPPY_VL_URL", "http://localhost:9428"),
		VictoriaMetricsURL: env("STROPPY_VM_URL", "http://localhost:8428"),
		VictoriaToken:      os.Getenv("STROPPY_VICTORIA_TOKEN"),
		ServerAddr_:        env("STROPPY_SERVER_ADDR", "http://localhost:8080"),
		AgentBinaryURL_:    env("STROPPY_AGENT_BINARY_URL", "http://localhost:8080/agent/stroppy-cloud"),
		AgentJWTTTL_:       envDuration("STROPPY_AGENT_JWT_TTL", 24*time.Hour),
		Agent: AgentConfig{
			ServerAddr: env("STROPPY_AGENT_SERVER", "localhost:8080"),
			TenantID:   os.Getenv("STROPPY_AGENT_TENANT"),
			MachineID:  env("STROPPY_MACHINE_ID", hostnameOr("agent")),
			Token:      os.Getenv("STROPPY_AGENT_TOKEN"),
			PollEvery:  envDuration("STROPPY_AGENT_POLL_EVERY", 3*time.Second),
		},
	}
	return c
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func hostnameOr(def string) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return def
}
