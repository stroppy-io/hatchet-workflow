package app

import "os"

// Config carries the wiring knobs for the demo App. Every field has a sane
// default so a zero-config `serve` works against a locally-running Temporal +
// Docker. Values come from the environment via LoadConfig.
type Config struct {
	// TemporalHostPort is the Temporal frontend address (default 127.0.0.1:7233).
	TemporalHostPort string
	// TemporalNamespace is the Temporal namespace (default "default").
	TemporalNamespace string
	// AgentImage is the docker image deployed for each topology instance
	// (default "stroppy-agent:latest").
	AgentImage string
	// StroppyUpstream is where the SERVER fetches the stroppy binary from (minio
	// or a github release). Agents never see this: they fetch stroppy through the
	// gateway (http://AgentServerAddr/artifacts/stroppy). May be empty.
	StroppyUpstream string
	// StroppyChecksum is the optional checksum for the stroppy binary.
	StroppyChecksum string
	// AgentGatewayAddr is the agent-facing gateway listener: Temporal proxy +
	// agent binary + artifact cache + apt proxy multiplexed on one port
	// (default :8080). This is the ONLY address agents are told about.
	AgentGatewayAddr string
	// AgentServerAddr is the FULL URL agents must dial the gateway at (scheme +
	// host + port, e.g. "http://host.docker.internal:8080" for local docker, or
	// the admin-set public address in production). Injected into every agent
	// container as STROPPY_SERVER_ADDR — the ONLY thing an agent (a "VM") knows.
	// In production this comes from the instance's PlatformSettings.ServerAddr
	// (admin-configured); empty there means derive the docker-host address.
	// TODO: read PlatformSettings.ServerAddr from the system_settings repo.
	AgentServerAddr string
	// AgentBinaryPath is the local path to the linux agent executable the gateway
	// serves at GET /agent/binary (so a fresh VM downloads its agent on boot).
	AgentBinaryPath string
	// CacheDir is where the gateway caches proxied artifacts.
	CacheDir string
	// AptCacheBackend is the internal apt-cacher-ng address the gateway forwards
	// agent apt traffic to (e.g. "apt-cacher-ng:3142"). Empty disables apt
	// forwarding. Agents reach it only through the gateway's single address.
	AptCacheBackend string
	// GRPCAddr is the listen address for the api gRPC server consumed by the UI /
	// e2e (default :8081; :8080 is the agent gateway).
	GRPCAddr string
	// DatabaseURL is the postgres DSN GORM connects to in Start (env DATABASE_URL).
	// Default targets the compose postgres at 127.0.0.1:5436 (db/user/pass stroppy).
	DatabaseURL string
}

// DefaultConfig returns the Config with all defaults applied.
func DefaultConfig() Config {
	return Config{
		TemporalHostPort:  "127.0.0.1:7233",
		TemporalNamespace: "default",
		AgentImage:        "stroppy-agent:latest",
		StroppyUpstream:   "",
		StroppyChecksum:   "",
		AgentGatewayAddr:  ":8080",
		AgentServerAddr:   "http://host.docker.internal:8080",
		AgentBinaryPath:   "",
		CacheDir:          "/var/lib/stroppy-cache/binaries",
		AptCacheBackend:   "",
		GRPCAddr:          ":8081",
		DatabaseURL:       "postgres://stroppy:stroppy@127.0.0.1:5436/stroppy?sslmode=disable",
	}
}

// LoadConfig builds a Config from the environment, falling back to defaults for
// any unset variable. AgentServerAddr defaults to AgentGatewayAddr when unset.
func LoadConfig() Config {
	cfg := DefaultConfig()
	cfg.TemporalHostPort = getenv("TEMPORAL_HOSTPORT", cfg.TemporalHostPort)
	cfg.TemporalNamespace = getenv("TEMPORAL_NAMESPACE", cfg.TemporalNamespace)
	cfg.AgentImage = getenv("AGENT_IMAGE", cfg.AgentImage)
	cfg.StroppyUpstream = getenv("STROPPY_UPSTREAM", cfg.StroppyUpstream)
	cfg.StroppyChecksum = getenv("STROPPY_CHECKSUM", cfg.StroppyChecksum)
	cfg.AgentGatewayAddr = getenv("AGENT_GATEWAY_ADDR", cfg.AgentGatewayAddr)
	cfg.AgentServerAddr = getenv("AGENT_SERVER_ADDR", cfg.AgentServerAddr)
	cfg.AgentBinaryPath = getenv("AGENT_BINARY_PATH", cfg.AgentBinaryPath)
	cfg.CacheDir = getenv("STROPPY_BINARY_CACHE_DIR", cfg.CacheDir)
	cfg.AptCacheBackend = getenv("STROPPY_APT_CACHE_BACKEND", cfg.AptCacheBackend)
	cfg.GRPCAddr = getenv("GRPC_ADDR", cfg.GRPCAddr)
	cfg.DatabaseURL = getenv("DATABASE_URL", cfg.DatabaseURL)
	return cfg
}

// getenv returns the environment value for key, or def when unset/empty.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
