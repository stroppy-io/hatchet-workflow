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
	// CatalogBundleDir is the local filesystem root for catalog entry bundles
	// (provider/workflow file sets — the SP-C seam's dev/staging BundleStore
	// impl; see internal/services/catalog.FSBundleStore).
	CatalogBundleDir string
	// AdminEmail is the email/login of the first-boot admin account seeded on a
	// brand-new database. Defaults to "admin@stroppy.local".
	AdminEmail string
	// AdminPassword is the plaintext password for the first-boot admin account.
	// When empty, first-boot seeding is skipped entirely (no password is
	// invented).
	AdminPassword string
	// GiteaBackend is the internal Gitea base URL (e.g. "http://gitea:3000")
	// the gateway reverse-proxies IDE git traffic to and internal/gitrepo's
	// bootstrap dials directly. Empty disables SP-C's git backend entirely —
	// no instance-repo bootstrap runs and gateway.Config.IdeBackend is left
	// unset (404 on /ide/*) — so existing deployments without Gitea
	// provisioned yet are unaffected. See docker-compose.yaml's gitea
	// service.
	GiteaBackend string
	// GiteaToken is the Gitea API token (SP-C's git backend credential),
	// sent as "Authorization: token <GiteaToken>". Provisioned out-of-band
	// (docker exec gitea gitea admin user generate-access-token — see
	// docker-compose.yaml's gitea service comment) and passed only via
	// environment; never committed, never baked into an image layer, and
	// never crosses a Temporal workflow-history boundary — the same posture
	// MonitoringToken already has in this Config.
	GiteaToken string
	// IdeBackend is a manual override: a single fixed code-server base URL
	// the gateway reverse-proxies every /ide/* request to, bypassing Task
	// 4's per-org manager entirely. Used only when IdeManagerEnabled is
	// false (e.g. pointing at an externally-run code-server during local
	// development); ignored otherwise (gateway.Config.IdeBackends takes
	// priority — see gateway.go).
	IdeBackend string
	// IdeManagerEnabled turns on Task 4's per-org code-server lifecycle
	// manager (internal/ide.Manager) — real docker containers, real
	// per-org/instance git worktrees, and the RBAC-backed IdeAuthorizer.
	// Requires GiteaToken (the manager clones/pulls from Gitea) and a
	// reachable docker daemon; both are checked at boot when this is true.
	// Defaults OFF: an operator opts in explicitly (IDE_MANAGER_ENABLED=1),
	// distinct from GiteaToken alone, because unlike GitBundleStore
	// (transparent storage swap) this spins up real containers.
	IdeManagerEnabled bool
	// IdeWorktreeRoot is the directory (inside the server's own container)
	// under which each org's (and the instance's) git worktree is
	// materialized, one subdirectory per scope.
	IdeWorktreeRoot string
	// IdeWorktreeVolume is the name of the Docker volume that backs
	// IdeWorktreeRoot in the server's own container (docker-compose.yaml's
	// `ide-worktrees`) — required so Manager can mount a scope's own
	// subdirectory (not the whole volume) into that scope's code-server
	// container via a volume-subpath mount rather than a host-path bind
	// mount that would resolve incorrectly under docker-outside-of-docker
	// (see internal/ide.Manager.Config.WorktreeVolume's doc). Empty is only
	// correct for bare/local (non-compose) usage.
	IdeWorktreeVolume string
	// IdeImage is the code-server image reference Manager starts per scope.
	IdeImage string
	// IdeDockerNetwork is the docker network code-server containers join so
	// the gateway's own container can reach them by name. Defaults to
	// AttachNetwork (the same network agent containers already join) when
	// empty.
	IdeDockerNetwork string
	// IdeLSPBinaryPath is a HOST path (as seen by dockerd, same caveat as
	// IdeWorktreeVolume) to the compiled cmd/stroppy-yaml-lsp binary.
	// Manager bind-mounts it read-only into every scope's code-server
	// container (internal/ide.Manager.Config.LSPBinaryPath) so a code-server
	// extension can spawn it over stdio — see
	// .superpowers/sdd/spc-t5-t7-report.md's "hosting" section. Empty (the
	// default) mounts nothing: a scope's code-server has no LSP available,
	// same as every deployment before Tasks 5-7.
	IdeLSPBinaryPath string
}
