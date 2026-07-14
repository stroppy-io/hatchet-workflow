package ide

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	"github.com/stroppy-io/stroppy-cloud/internal/gitrepo"
)

// RepoEnsurer is the subset of *internal/gitrepo.Client Manager needs to
// guarantee an entry's repo exists before cloning it — a narrow interface
// for the same reason catalog.GitClient is (testable with a fake; *gitrepo.
// Client already satisfies it structurally).
type RepoEnsurer interface {
	EnsureOrg(ctx context.Context, org string) error
	EnsureRepo(ctx context.Context, owner, name string, private bool) error
}

// containerNameRe matches the characters Docker allows in a container name;
// scope keys are turned into container names by substituting anything else,
// so a hostile org slug can never inject shell/path metacharacters into a
// container name string.
var containerNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// Manager ensures exactly one running code-server container per scope
// (spec §9.2's decided "shared per-tenant code-server", not per-user) and
// keeps its bind-mounted workspace synced to the scope's git repo via
// EnsureWorktree. One Manager serves every scope — isolation between orgs
// is NOT "one Manager per org" but two narrower guarantees this type
// enforces on every call:
//   - the container name and the worktree directory are both derived from
//     scope.Key() alone (containerName/worktreeDir below), so two different
//     scopes can never collide onto the same container or the same
//     directory;
//   - EnsureRunning never bind-mounts anything but worktreeRoot/<scope.Key()>
//     into the container it starts for that scope, so a code-server
//     container started for org "acme" has no filesystem path into any
//     other org's or the instance's worktree.
type Manager struct {
	docker         *client.Client
	gitea          RepoEnsurer
	giteaBaseURL   string
	giteaToken     string
	worktreeRoot   string
	worktreeVolume string
	image          string
	network        string
	lspBinaryPath  string

	// mu guards entryLocks; entryLocks serializes materialization per entry so
	// a burst of requests for a cold editor produces one clone, not a race.
	mu         sync.Mutex
	entryLocks map[string]*sync.Mutex
}

// lockEntry takes the per-entry materialization lock and returns its release.
func (m *Manager) lockEntry(key string) func() {
	m.mu.Lock()
	if m.entryLocks == nil {
		m.entryLocks = map[string]*sync.Mutex{}
	}
	l, ok := m.entryLocks[key]
	if !ok {
		l = &sync.Mutex{}
		m.entryLocks[key] = l
	}
	m.mu.Unlock()

	l.Lock()
	return l.Unlock
}

// Config configures a Manager.
type Config struct {
	Docker *client.Client
	// Gitea ensures an org's repo exists before Manager clones it (nil is
	// valid only when every scope Manager will ever see is ScopeInstance,
	// whose repo Bootstrap already guarantees — see internal/gitrepo).
	Gitea RepoEnsurer
	// GiteaBaseURL/GiteaToken build the git-smart-HTTP remote (RemoteURL) for
	// clone/pull.
	GiteaBaseURL string
	GiteaToken   string
	// WorktreeRoot is the directory (as seen from wherever THIS process's
	// EnsureWorktree/git clone-pull runs — see the WorktreeVolume doc for why
	// that is not necessarily the same filesystem view the code-server
	// container gets) under which each scope gets its own subdirectory
	// (WorktreeRoot/<scope.Key()>).
	WorktreeRoot string
	// WorktreeVolume, when set, is the name of a Docker VOLUME (not a host
	// path) that already backs WorktreeRoot in THIS process's own container
	// (docker-compose.yaml's `ide-worktrees:/var/lib/stroppy-ide`, for the
	// deployed case where the server itself runs inside a container talking
	// to the host's dockerd over a mounted docker.sock — the classic
	// "docker-outside-of-docker" setup this codebase already uses for agent
	// containers). When set, EnsureRunning mounts a scope's container onto
	// that SAME named volume with mount.VolumeOptions.Subpath scoped to the
	// scope's own subdirectory, rather than a bind mount of WorktreeRoot as
	// a literal host path — a bind mount naming a path inside the server
	// container's OWN filesystem would resolve on the HOST's filesystem when
	// dockerd creates the sibling code-server container, which is never the
	// same path and would silently mount the wrong (usually nonexistent)
	// directory. Empty (bare/local, non-compose usage — e.g. `go run`
	// against a local docker daemon where WorktreeRoot genuinely IS a host
	// path) falls back to a plain bind mount.
	WorktreeVolume string
	// Image is the code-server image reference (e.g.
	// "codercom/code-server:4.96.4").
	Image string
	// Network is the docker network code-server containers join, so the
	// gateway's server container can reach them by container name — same
	// network agent containers attach to (AGENT_ATTACH_NETWORK).
	Network string
	// LSPBinaryPath, when set, is a HOST path (as seen by whatever process
	// runs dockerd — see the WorktreeVolume doc for the same
	// docker-outside-of-docker caveat) to the compiled cmd/stroppy-yaml-lsp
	// binary. EnsureRunning bind-mounts it READ-ONLY into every scope's
	// code-server container at lspBinaryContainerPath, so a code-server
	// extension inside the container can spawn it as a stdio subprocess —
	// see .superpowers/sdd/spc-t5-t7-report.md's "hosting" section. Empty
	// (the default) mounts nothing; a scope's container simply has no LSP
	// available, exactly like every deployment before this task.
	LSPBinaryPath string
}

// lspBinaryContainerPath is the fixed path inside every scope's code-server
// container LSPBinaryPath (when configured) is mounted at — a code-server
// extension's serverOptions.command points here, a plain constant rather
// than a per-scope value, since the binary itself carries no scope-specific
// state (DslService is instantiated fresh per LSP process; scope isolation
// is enforced by internal/ide/lsp.Session.ResolvePath against the
// workspace root the extension passes at `initialize`, not by which binary
// path is used).
const lspBinaryContainerPath = "/usr/local/bin/stroppy-yaml-lsp"

// maxContainerKeyLen bounds the readable half of a container name so the
// whole thing (prefix + key + "-" + 8-hex hash) stays inside DNS's 63-char
// label limit.
const maxContainerKeyLen = 40

// Cold-start readiness bounds for waitReady: code-server takes a couple of
// seconds to bind after docker reports the container running.
const (
	readyTimeout      = 60 * time.Second
	readyPollInterval = 250 * time.Millisecond
	dialTimeout       = 2 * time.Second
)

// workspaceContainerPath is where a scope's worktree is mounted inside its
// code-server container, and the folder code-server is told to open.
const workspaceContainerPath = "/home/coder/project"

// NewManager builds a Manager from cfg.
func NewManager(cfg Config) *Manager {
	return &Manager{
		docker:         cfg.Docker,
		gitea:          cfg.Gitea,
		giteaBaseURL:   cfg.GiteaBaseURL,
		giteaToken:     cfg.GiteaToken,
		worktreeRoot:   cfg.WorktreeRoot,
		worktreeVolume: cfg.WorktreeVolume,
		image:          cfg.Image,
		network:        cfg.Network,
		lspBinaryPath:  cfg.LSPBinaryPath,
	}
}

// containerName names the ONE code-server container a workspace gets. A
// docker container name doubles as its DNS label on the network, and DNS
// caps a label at 63 characters — an org key carries a 36-char tenant UUID,
// so the raw form overflowed and docker's resolver answered NXDOMAIN while
// the editor sat there healthy. Hash the variable half; the prefix stays
// readable.
func containerName(workspaceKey string) string {
	sanitized := containerNameRe.ReplaceAllString(workspaceKey, "-")
	if len(sanitized) > maxContainerKeyLen {
		sum := sha256.Sum256([]byte(workspaceKey))
		sanitized = sanitized[:maxContainerKeyLen] + "-" + hex.EncodeToString(sum[:])[:8]
	}
	return "stroppy-ide-" + sanitized
}

// worktreeSubpath is the scope-specific directory name, relative to
// worktreeRoot — used both to build worktreeDir (this process's own git
// clone/pull target) and, when worktreeVolume is set, as
// mount.VolumeOptions.Subpath for the sibling container (see the
// WorktreeVolume config doc for why those two are not the same path).
func worktreeSubpath(key string) string {
	return containerNameRe.ReplaceAllString(key, "-")
}

func (m *Manager) worktreeDir(key string) string {
	return m.worktreeRoot + "/" + worktreeSubpath(key)
}

// giteaOwnerRepo returns the (owner, repo) EnsureRunning materializes for
// scope — the SAME (owner, repo) internal/services/catalog.GitEntryBundleStore
// would compute for the equivalent CatalogEntry (gitrepo.OwnerFor +
// gitrepo.EntryRepoName), so the IDE opens exactly the entry's one real
// catalog repo, never an org-wide directory — and ensures it exists first
// (an instance entry may already exist via a prior catalog Write; an org
// entry's repo is created here on first IDE open too, mirroring
// GitEntryBundleStore.ensureEntryRepo).
func (m *Manager) giteaOwnerRepo(ctx context.Context, scope Scope) (owner, repo string, err error) {
	if scope.EntryKind == "" || scope.EntrySlug == "" {
		return "", "", fmt.Errorf("ide: scope %q names no catalog entry (missing kind/slug)", scope.Key())
	}
	if scope.EntryKind == EntryKindRecipe && scope.Kind == ScopeInstance {
		// Defense in depth: Authorizer.CanAuthor already rejects this scope
		// shape unconditionally (see its own doc — a recipe is always
		// tenant-owned), but EnsureRunning must never materialize an
		// "instance recipe repo" even if some future caller reached here
		// without going through the authorizer.
		return "", "", fmt.Errorf("ide: recipe scope has no instance level")
	}
	switch scope.Kind {
	case ScopeInstance:
		owner = gitrepo.InstanceOrg
	case ScopeOrg:
		if scope.OrgSlug == "" {
			return "", "", fmt.Errorf("ide: org scope has no tenant id")
		}
		// scope.OrgSlug here is whatever the caller (the IdeBackendResolver's
		// resolved tenant id, NOT the raw URL slug — see backend.go) passed
		// through as the scope; EnsureRunning's caller is responsible for
		// having already turned an untrusted URL slug into a real tenant id
		// before it ever reaches Manager. gitrepo.TenantOrg further sanitizes
		// it defensively (see its own doc).
		owner = gitrepo.TenantOrg(scope.OrgSlug)
	default:
		return "", "", fmt.Errorf("ide: unrecognized scope kind %v", scope.Kind)
	}
	if m.gitea == nil {
		return "", "", fmt.Errorf("ide: manager has no RepoEnsurer, cannot materialize an entry worktree")
	}
	repo = gitrepo.EntryRepoName(scope.EntryKind, scope.EntrySlug)
	if err := m.gitea.EnsureOrg(ctx, owner); err != nil {
		return "", "", fmt.Errorf("ide: ensure org %q: %w", owner, err)
	}
	if err := m.gitea.EnsureRepo(ctx, owner, repo, true); err != nil {
		return "", "", fmt.Errorf("ide: ensure entry repo %q/%q: %w", owner, repo, err)
	}
	return owner, repo, nil
}

// EnsureRunning materializes scope's worktree (clone-or-pull) and starts (or
// confirms already-running) its code-server container, returning the
// container's internal HTTP base URL for gateway.IdeBackendResolver to
// reverse-proxy to.
func (m *Manager) EnsureRunning(ctx context.Context, scope Scope) (string, error) {
	name := containerName(scope.WorkspaceKey())

	// Fast path. The gateway resolves a backend for EVERY proxied request, and
	// a code-server page load is dozens of static assets in parallel. Doing the
	// gitea round-trip and a `git pull` on each of them meant they raced for
	// the worktree's index.lock and most of them lost: 18 of 20 concurrent
	// asset requests came back 503. Once the container is up there is nothing
	// to materialize — hand back its address and touch neither git nor docker's
	// create path.
	if insp, err := m.docker.ContainerInspect(ctx, name); err == nil &&
		insp.State != nil && insp.State.Running {
		return containerBaseURL(name), nil
	}

	// Slow path: at most one materialization per entry at a time, so a burst of
	// requests for a not-yet-started editor still results in exactly one clone
	// and one container.
	unlock := m.lockEntry(scope.Key())
	defer unlock()

	// Another request may have finished the whole thing while we waited.
	if insp, err := m.docker.ContainerInspect(ctx, name); err == nil &&
		insp.State != nil && insp.State.Running {
		return containerBaseURL(name), nil
	}

	owner, repo, err := m.giteaOwnerRepo(ctx, scope)
	if err != nil {
		return "", err
	}
	remote, err := RemoteURL(m.giteaBaseURL, owner, repo)
	if err != nil {
		return "", fmt.Errorf("ide: remote url: %w", err)
	}
	// One workspace = one worktree directory = one container (spec §9.2).
	// The entry's own repo is cloned into a subdirectory of it, side by side
	// with every other entry of the same workspace, and the browser opens the
	// one it wants via code-server's ?folder= deep link. Isolation is per
	// WORKSPACE — a tenant's container can still never see another tenant's
	// (or the instance's) worktree.
	dest := m.worktreeDir(scope.WorkspaceKey()) + "/" + scope.EntryDir()
	if _, err := EnsureWorktree(ctx, remote, m.giteaToken, dest); err != nil {
		return "", fmt.Errorf("ide: materialize worktree for %q: %w", scope.Key(), err)
	}

	if insp, err := m.docker.ContainerInspect(ctx, name); err == nil {
		if insp.State != nil && insp.State.Running {
			return containerBaseURL(name), nil
		}
		// Exists but stopped: start it rather than re-create (the bind mount
		// spec is unchanged since the container still exists).
		if err := m.docker.ContainerStart(ctx, insp.ID, container.StartOptions{}); err != nil {
			return "", fmt.Errorf("ide: restart container %q: %w", name, err)
		}
		if err := m.waitReady(ctx, name); err != nil {
			return "", err
		}
		return containerBaseURL(name), nil
	}

	mounts := []mount.Mount{m.workspaceMount(scope.WorkspaceKey(), m.worktreeDir(scope.WorkspaceKey()))}
	if m.lspBinaryPath != "" {
		mounts = append(mounts, m.lspBinaryMount())
	}
	hostCfg := &container.HostConfig{
		Mounts: mounts,
	}
	if m.network != "" {
		hostCfg.NetworkMode = container.NetworkMode(m.network)
	}
	cfg := &container.Config{
		Image: m.image,
		Env: []string{
			// Auth is enforced at the gateway boundary by IdeAuthorizer BEFORE
			// a request ever reaches this container (spec §3 C4) — code-server's
			// own password prompt would be redundant defense-in-depth at best
			// and an extra credential to provision at worst, so it is disabled.
			// This container is reachable ONLY on the internal docker network
			// (no published port — see hostCfg above), never directly from the
			// public internet.
			"PASSWORD=",
		},
		// An empty PASSWORD does NOT disable code-server's login: it falls back
		// to the password in its own generated ~/.config/code-server/config.yaml
		// and 302s every request to ./login, which the gateway then proxies as a
		// broken redirect. `--auth none` is the actual off switch.
		//
		// The worktree is passed as the folder to open, so a session lands on the
		// catalog entry's repo root instead of an empty $HOME.
		Cmd: []string{
			"--auth", "none",
			"--bind-addr", "0.0.0.0:" + codeServerPort,
			workspaceContainerPath,
		},
	}
	var netCfg *network.NetworkingConfig
	if m.network != "" {
		netCfg = &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{
			m.network: {},
		}}
	}
	resp, err := m.docker.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
	if err != nil {
		return "", fmt.Errorf("ide: create container %q: %w", name, err)
	}
	if err := m.docker.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("ide: start container %q: %w", name, err)
	}
	if err := m.waitReady(ctx, name); err != nil {
		return "", err
	}
	return containerBaseURL(name), nil
}

// waitReady blocks until code-server inside name accepts a TCP connection, or
// the deadline passes. Without it EnsureRunning hands the gateway a backend
// address the instant docker reports the container started — but code-server
// needs a few seconds to bind, so the very first request after a cold start
// proxied into a refused connection and the browser got a 502. Retrying by
// hand happened to work, which is exactly what made this look like "the
// container is just slow" instead of a race.
func (m *Manager) waitReady(ctx context.Context, name string) error {
	addr := net.JoinHostPort(name, codeServerPort)
	deadline := time.Now().Add(readyTimeout)
	var lastErr error
	for {
		conn, err := net.DialTimeout("tcp", addr, dialTimeout)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("ide: code-server %q did not become ready in %s: %w", name, readyTimeout, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPollInterval):
		}
	}
}

// Stop removes scope's code-server container. Idempotent: removing an
// absent container is not an error (mirrors deleteEntry's idempotency
// convention elsewhere in this codebase). The worktree directory on disk is
// left in place — Stop is a container-lifecycle op, not a data-deletion op.
func (m *Manager) Stop(ctx context.Context, scope Scope) error {
	name := containerName(scope.WorkspaceKey())
	err := m.docker.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
	if err != nil && !strings.Contains(err.Error(), "No such container") {
		return fmt.Errorf("ide: remove container %q: %w", name, err)
	}
	return nil
}

// codeServerPort is the port EnsureRunning tells code-server to bind
// (--bind-addr above). The gateway proxies to the container by name on this
// port; nothing is published to the host.
const codeServerPort = "8080"

func containerBaseURL(name string) string {
	return "http://" + name + ":" + codeServerPort
}

// workspaceMount builds the mount.Mount for scope's code-server container's
// /home/coder/project — a docker-volume subpath mount when worktreeVolume
// is configured (the deployed/compose case, see WorktreeVolume's doc),
// otherwise a plain bind mount of dest (the bare/local-docker case).
func (m *Manager) workspaceMount(scopeKey, dest string) mount.Mount {
	if m.worktreeVolume != "" {
		return mount.Mount{
			Type:   mount.TypeVolume,
			Source: m.worktreeVolume,
			Target: workspaceContainerPath,
			VolumeOptions: &mount.VolumeOptions{
				Subpath: worktreeSubpath(scopeKey),
			},
		}
	}
	return mount.Mount{
		Type:   mount.TypeBind,
		Source: dest,
		Target: workspaceContainerPath,
	}
}

// lspBinaryMount bind-mounts the host's compiled stroppy-yaml-lsp binary
// read-only into every scope's container at lspBinaryContainerPath — always
// a bind (never a docker-outside-of-docker volume-subpath concern like
// workspaceMount's): the binary is a single static file built once by the
// image/deploy pipeline at a fixed HOST path, not a per-scope subtree that
// needs the same volume-subpath trick worktrees do.
func (m *Manager) lspBinaryMount() mount.Mount {
	return mount.Mount{
		Type:     mount.TypeBind,
		Source:   m.lspBinaryPath,
		Target:   lspBinaryContainerPath,
		ReadOnly: true,
	}
}
