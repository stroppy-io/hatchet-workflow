// Package app is the integration layer: it wires the postgres store, the
// identity/execution/adapter infrastructure, all 20 control-plane services, the
// Temporal server worker and the agent gateway into one running control plane.
//
// app.Run owns the whole object graph and lifecycle; cmd/cli only constructs the
// Config (from env) and calls Run.
package app

// Config is the control-plane runtime configuration. Every field is a plain
// string sourced by cmd/cli from the environment; Run applies defaults and
// derives the typed dependencies from these values.
type Config struct {
	// DatabaseURL is the postgres DSN the store connects to.
	DatabaseURL string
	// JWTSecret is the HMAC signing secret for access/refresh tokens.
	JWTSecret string
	// TemporalHostPort is the Temporal frontend (host:port) the client + worker
	// dial and the gateway proxies to.
	TemporalHostPort string
	// TemporalNS is the Temporal namespace.
	TemporalNS string
	// ListenAddr is the single gateway listen address (connect API + SPA + agent
	// routes + Temporal proxy share it via cmux).
	ListenAddr string
	// AgentServerAddr is the public address provisioned agents reach the server at
	// (baked into the AgentBootstrap).
	AgentServerAddr string
	// AttachNetwork is the docker network agents attach to (provisioning hint).
	AttachNetwork string
	// AgentImage is the agent container image.
	AgentImage string
	// AgentBinaryPath is the local agent executable served at GET /agent/binary.
	AgentBinaryPath string
	// CacheDir is where the gateway caches proxied binaries + apt packages.
	CacheDir string
	// StroppyUpstream is the upstream URL for the "stroppy" artifact.
	StroppyUpstream string
	// StroppyGitHubRepo is the GitHub repository used to list suggested Stroppy
	// release versions ("owner/name"). Empty defaults to "stroppy-io/stroppy".
	StroppyGitHubRepo string
	// StroppyMinVersion is the inclusive minimum release version shown by
	// ListStroppyVersions. Empty disables the floor.
	StroppyMinVersion string
	// StroppyGitHubToken is an optional GitHub token for release listing API
	// limits.
	StroppyGitHubToken string
	// QuotaRefreshInterval controls the periodic provider quota snapshot refresh.
	// Empty defaults to 5m; "0" disables the background refresher.
	QuotaRefreshInterval string
	// QuotaSnapshotTTL controls when cached provider quota snapshots are stale.
	// Empty defaults to 5m.
	QuotaSnapshotTTL string
	// QuotaReservationTTL controls how long pre-provision reservations can stay
	// active before they expire. Empty defaults to 30m.
	QuotaReservationTTL string
	// AptBackend is the apt-cacher-ng backend the gateway relays agent apt traffic
	// to.
	AptBackend string
	// MonitoringURL is the metrics/logs backend root (VictoriaMetrics/Logs /
	// vmauth). The gateway relays agent /insert/* + /select/* traffic here.
	MonitoringURL string
	// MonitoringToken is the bearer token agents present to the gateway for
	// monitoring relay + Temporal proxy access; the gateway also injects it on
	// every relayed monitoring request so it authenticates to vmauth.
	MonitoringToken string
	// GrafanaBackend is the embedded Grafana upstream the gateway reverse-proxies
	// /grafana/* to, serving dashboards from the same server origin.
	GrafanaBackend string
	// RegistryBackend is the Docker registry upstream the gateway reverse-proxies
	// /v2/* to, so agents can pull images without direct internet access.
	// Defaults to http://registry:5000 (STROPPY_REGISTRY_BACKEND env var).
	RegistryBackend string
	// PackageBlobDir is the local filesystem root for package blobs.
	PackageBlobDir string
	// AdminEmail is the email/login of the first-boot admin account seeded on a
	// brand-new database. Defaults to "admin@stroppy.local".
	AdminEmail string
	// AdminPassword is the plaintext password for the first-boot admin account.
	// When empty, first-boot seeding is skipped entirely (no password is
	// invented).
	AdminPassword string
}
