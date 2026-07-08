# SP-C: Internal git + встроенная IDE — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the product an internal git backend (Gitea sidecar) with instance/org repositories, a per-org code-server IDE proxied through the existing gateway, and a `stroppy-yaml` LSP extension that is a thin client over the already-existing `DslService.Check`/`ComposedSchema`/`Preview` RPCs — replacing zero of the compiler, only adding transport/formatting on top of it.

**Architecture:**
- `internal/gitrepo` (new package) — a hand-rolled Gitea REST API client (`net/http`, injectable `BaseURL`/`HTTPClient`, same shape as `internal/infrastructure/adapters.GitHubStroppyVersionSource`) plus an idempotent instance-repo bootstrap run at server startup.
- `internal/gateway` — one new `Config.IdeBackend` field + one new `serveHTTP` branch (`/ide/*`), built with the existing `newMonitorProxy` single-host reverse-proxy constructor, exactly like `GrafanaBackend`/`RegistryBackend`. A new `IdeAuthorizer` interface (nil-disables, mirroring `AgentTokenVerifier`'s existing convention in this same package) gates the route before it ever reaches code-server; SP-B supplies the real RBAC-backed implementation later, this plan only cuts the seam.
- `internal/ide` (new package) — per-org code-server container lifecycle, built on `github.com/docker/docker/client` the same way `internal/infrastructure/docker.Executor` already drives agent containers (`ContainerCreate`/`ContainerStart`/`ContainerRemove`), plus a worktree materializer that clones/pulls an org's Gitea repo onto a host path bind-mounted into the container.
- `internal/ide/lsp` (new package) — an in-process Go LSP server (chosen over a networked connect-client per spec §4's open transport question — see Global Constraints) that wraps `internal/services/dsl.DslService.Check/ComposedSchema/Preview` directly (no RPC hop; same process family as the server, per spec §4 "In-process" option). Ships as a small binary (`cmd/stroppy-yaml-lsp`) the code-server extension spawns over stdio, per LSP convention.
- `docker-compose.yaml` — one new `gitea` sidecar service (`+healthcheck`), the same sidecar pattern already used for `grafana`/`registry`/`vmauth`. Code-server containers are **not** static compose services (they are per-org and dynamic) — they are spun up by `internal/ide`'s manager, analogous to how docker recipe runs already spin up agent containers dynamically rather than being compose entries.

**Tech Stack:** Go, `net/http` (Gitea REST client, no SDK dependency), `github.com/docker/docker/client` (already a direct dependency via `internal/infrastructure/docker`), `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2` (new dependency — LSP wire types + JSON-RPC framing; install via `go get` per repo convention, do not hand-roll LSP framing), connect-rpc (existing `dslconnect` package, unused by this plan directly since the LSP is in-process — kept available for a future non-Go extension host), Gitea `api/v1` REST API (`docker.io/library` image `gitea/gitea`, exact tag pinned at implementation time).

## Global Constraints

- No co-author footer in commits. Conventional Commits (`type(scope): summary`).
- SP-C does **not** implement SP-B (Catalog/RBAC). Every place SP-C needs "does this principal have authoring rights on this repo" is cut as a narrow Go interface (`gateway.IdeAuthorizer`, `internal/gitrepo`'s caller-supplied ACL check) that SP-C wires with a permissive/test double; SP-B supplies the real implementation in its own plan. Do not build a fake RBAC system to fill the gap.
- `internal/services/dsl.DslService.Check/ComposedSchema/Preview` are **not modified** by this plan — the LSP is purely a new client of the existing three RPCs, in-process (no new RPC, no new field on the request/response messages this plan can't already get from `Files map[string][]byte`).
- `ComposedSchema` today returns JSON-Schema (`SchemaJson string`, `internal/dsl/schema.Compose`'s jsonschema output) — SP-A's plan (`2026-07-08-sp-a-schema-layer.md`) rewires it to `schemapb.Schema` protojson but has **not landed** as of this plan. Task 6 (LSP completion) is therefore written against **today's** JSON-Schema shape (`properties`/`type`/`enum`/`description`) and explicitly flagged as the pre-SP-A degraded path (spec §7); swapping to `protojson.Unmarshal` into `*schemapb.Schema` is a one-function change (`parseComposedSchema` in Task 6) once SP-A lands — noted inline, not a separate task here.
- `RecipeEditor.tsx`/`dsl-editor.tsx` (the CodeMirror lite-path) are **not modified** by this plan (spec §2 "Вне scope").
- Gitea/code-server are new deployment-time dependencies — most of C1/C2/C4's correctness is tested against `httptest` mocks that speak the same REST/HTTP shape (see each task's Testable-without-live-infra note), not a live Gitea/code-server container. A short manual/E2E verification list closes the gap; see Self-Review.
- Docker container lifecycle code (Task 4) reuses `internal/infrastructure/docker`'s helpers (`hostConfig`, `containerConfig`, `envList`, etc.) rather than re-deriving container spec construction from scratch — import and call them, don't copy-paste.

---

### Task 1: Gitea sidecar + instance-repo bootstrap

Add the Gitea sidecar to the compose stack and a Go client with the minimal repo-lifecycle primitives (`EnsureOrg`, `EnsureRepo`), wired into server startup to guarantee the singleton **instance** repo exists on every boot. Org-repo creation is a capability this task exposes, not a flow it drives (SP-B's org-creation flow calls `EnsureRepo` later — untested here beyond the function itself, since no org-creation caller exists yet).

**Files:**
- Create: `internal/gitrepo/client.go`
- Create: `internal/gitrepo/client_test.go`
- Create: `internal/gitrepo/bootstrap.go`
- Create: `internal/gitrepo/bootstrap_test.go`
- Modify: `docker-compose.yaml` (new `gitea` service)
- Modify: `internal/app/config.go` (new `GiteaBackend`/`GiteaToken`/`GiteaInstanceRepo` fields, same doc-comment style as `GrafanaBackend`)
- Modify: `cmd/cli/serve_cmd.go` (env wiring, same `env("X", default)` helper already used for `STROPPY_REGISTRY_BACKEND`)
- Modify: `internal/app/run.go` (call `gitrepo.Bootstrap` before `gateway.New`, fail-fast like the existing postgres/temporal startup checks)
- Reference (pattern to mirror, do not modify): `internal/infrastructure/adapters/stroppy_versions.go` (injectable `BaseURL`/`HTTPClient` REST client shape), `docker-compose.yaml`'s `grafana`/`vmauth` services (sidecar + healthcheck pattern)

**Interfaces:**
```go
// internal/gitrepo/client.go
type Config struct {
    BaseURL    string        // e.g. "http://gitea:3000"; injectable for tests
    Token      string        // Gitea API token ("Authorization: token <Token>")
    HTTPClient *http.Client  // nil -> &http.Client{Timeout: 10*time.Second}
}

type Client struct { /* unexported baseURL, token, http */ }

func NewClient(cfg Config) (*Client, error)

// EnsureOrg creates org if it does not already exist (idempotent: a 422
// "already exists" from Gitea's create-org endpoint is treated as success).
func (c *Client) EnsureOrg(ctx context.Context, org string) error

// EnsureRepo creates owner/name if it does not already exist (idempotent,
// same 422-swallow convention). owner is either "" (create under the token's
// own user — used for the instance repo) or an org name (EnsureOrg first).
func (c *Client) EnsureRepo(ctx context.Context, owner, name string, private bool) error
```
```go
// internal/gitrepo/bootstrap.go
const InstanceRepoOwner = "" // instance repo lives under the service account user
const InstanceRepoName = "instance-catalog"

// Bootstrap ensures the singleton instance repo exists. Called once at
// server startup (internal/app/run.go); errors here fail server boot the
// same way an unreachable postgres does — the instance repo is load-bearing
// for the whole catalog, not an optional feature.
func Bootstrap(ctx context.Context, c *Client) error
```

- [ ] **Step 1: Write the failing test**

```go
// internal/gitrepo/client_test.go
package gitrepo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureRepo_CreatesUnderOrg(t *testing.T) {
	var gotPath, gotMethod, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod, gotAuth = r.URL.Path, r.Method, r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"name":"acme"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "gitea-tok"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if err := c.EnsureRepo(context.Background(), "acme-org", "acme", true); err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	if got, want := gotMethod, http.MethodPost; got != want {
		t.Fatalf("method = %s, want %s", got, want)
	}
	if got, want := gotPath, "/api/v1/orgs/acme-org/repos"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
	if got, want := gotAuth, "token gitea-tok"; got != want {
		t.Fatalf("authorization = %s, want %s", got, want)
	}
}

func TestEnsureRepo_AlreadyExistsIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"repository already exists"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.EnsureRepo(context.Background(), "acme-org", "acme", true); err != nil {
		t.Fatalf("ensure repo should be idempotent, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitrepo/... -run TestEnsureRepo -v`
Expected: FAIL — package `gitrepo` / `NewClient` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/gitrepo/client.go
package gitrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("gitrepo: empty BaseURL")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: base, token: cfg.Token, http: hc}, nil
}

func (c *Client) EnsureOrg(ctx context.Context, org string) error {
	body := map[string]string{"username": org}
	return c.postIdempotent(ctx, "/api/v1/orgs", body)
}

func (c *Client) EnsureRepo(ctx context.Context, owner, name string, private bool) error {
	path := "/api/v1/user/repos"
	if owner != "" {
		path = fmt.Sprintf("/api/v1/orgs/%s/repos", owner)
	}
	body := map[string]any{"name": name, "private": private, "auto_init": true}
	return c.postIdempotent(ctx, path, body)
}

// postIdempotent POSTs body and treats Gitea's 422 "already exists" response
// as success — EnsureOrg/EnsureRepo are called on every server boot / every
// org-creation flow invocation, never guarded by a prior existence check.
func (c *Client) postIdempotent(ctx context.Context, path string, body any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusUnprocessableEntity && strings.Contains(string(respBody), "already exists") {
		return nil
	}
	return fmt.Errorf("gitea %s %s: %s: %s", http.MethodPost, path, resp.Status, strings.TrimSpace(string(respBody)))
}
```

> NOTE: verify the exact 422 message text against the deployed Gitea version at implementation time (Gitea's wording has changed across major versions) — this is the same class of caveat SP-A's plan flagged for `tfconfig`'s validation-block field.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitrepo/... -run TestEnsureRepo -v`
Expected: PASS.

- [ ] **Step 5: Bootstrap + startup wiring test**

```go
// internal/gitrepo/bootstrap_test.go
func TestBootstrap_CreatesInstanceRepoUnderUserRepos(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := Bootstrap(context.Background(), c); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if got, want := gotPath, "/api/v1/user/repos"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
}
```

Implement `Bootstrap` as a thin call to `c.EnsureRepo(ctx, InstanceRepoOwner, InstanceRepoName, true)`.

Run: `go test ./internal/gitrepo/... -v` → PASS.

- [ ] **Step 6: Compose + config wiring (not unit-tested — infra config)**

Add to `docker-compose.yaml`, mirroring the `grafana`/`vmauth` sidecar shape:

```yaml
  gitea:
    image: gitea/gitea:1.23  # pin exact patch at implementation time
    restart: always
    environment:
      GITEA__server__ROOT_URL: http://gitea:3000/
      GITEA__server__DISABLE_SSH: "true"
      GITEA__service__DISABLE_REGISTRATION: "true"
      GITEA__security__INSTALL_LOCK: "true"
    volumes:
      - giteadata:/data
    healthcheck:
      test: [ "CMD-SHELL", "curl -sf http://localhost:3000/api/healthz || exit 1" ]
      interval: 5s
      timeout: 3s
      retries: 10
```

Add `giteadata:` to the `volumes:` block; add `gitea: condition: service_healthy` to `server`'s `depends_on:`. Add `GITEA_BACKEND`/`GITEA_TOKEN` env vars to the `server` service (a fixed API token created via Gitea's `gitea admin user generate-access-token` at image-build/first-boot time — exact provisioning mechanism is an implementation-time detail, not fixed by this plan; a static `GITEA_ADMIN_TOKEN` env baked at compose-up via an init container/entrypoint script is the simplest v1 option). Add `GiteaBackend string` / `GiteaToken string` to `internal/app/config.go` (doc-comment style matching `GrafanaBackend`), env wiring in `cmd/cli/serve_cmd.go` (`env("GITEA_BACKEND", "http://gitea:3000")`), and a `gitrepo.Bootstrap` call in `internal/app/run.go` before `gateway.New(...)`, returning a wrapped error on failure (fail server boot, same severity as an unreachable postgres).

**Testable without live infra:** Steps 1–5 (Go client + bootstrap logic) — full coverage via `httptest`. **Requires live infra:** Step 6's compose healthcheck and the actual admin-token provisioning mechanism — verify manually with `docker compose up gitea` + `curl http://localhost:3000/api/healthz` before wiring the token into `server`.

- [ ] **Step 7: Commit**

```bash
git add internal/gitrepo/client.go internal/gitrepo/client_test.go internal/gitrepo/bootstrap.go internal/gitrepo/bootstrap_test.go docker-compose.yaml internal/app/config.go cmd/cli/serve_cmd.go internal/app/run.go
git commit -m "feat(gitrepo): add Gitea sidecar + instance-repo bootstrap"
```

---

### Task 2: Git content adapter (commit bundle, read file/tree)

Extend `internal/gitrepo.Client` with the content operations SP-C's own consumers need: committing a bundle's `files map[string][]byte` as a single logical write, and reading a file/tree back at a ref. This is the same `files`-shaped contract `DslService.Check/ComposedSchema/Preview` already accept — a git ref is just another way to produce that map.

**Files:**
- Modify: `internal/gitrepo/client.go` (add `CommitFiles`, `GetFile`, `ListTree`)
- Modify: `internal/gitrepo/client_test.go`

**Interfaces:**
```go
// CommitFiles writes every entry of files to owner/repo on branch, one Gitea
// contents-API call per file (Gitea's contents endpoint is single-file; a
// true atomic multi-file commit needs Gitea's "diff patch" API — deferred,
// see Self-Review). author/message become the commit's author/message; a
// pre-existing file is updated (fetches its blob sha first, per Gitea's
// contents API requiring sha on update), a new file is created.
func (c *Client) CommitFiles(ctx context.Context, owner, repo, branch string, files map[string][]byte, author, email, message string) error

// GetFile reads path's content at ref (branch/tag/commit sha).
func (c *Client) GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error)

// ListTree lists every blob path under ref, recursively — the files map
// DslService.Check/ComposedSchema/Preview expect, keyed the same way
// include.Sources already keys a bundle (slash-separated logical path).
func (c *Client) ListTree(ctx context.Context, owner, repo, ref string) ([]string, error)
```

- [ ] **Step 1: Write the failing test**

```go
func TestGetFile_DecodesBase64Content(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/repos/acme/wf/contents/cluster.yaml"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.URL.Query().Get("ref"), "main"; got != want {
			t.Fatalf("ref = %s, want %s", got, want)
		}
		w.Write([]byte(`{"content":"cHJvdmlkZXI6CiAgdXNlOiBkb2NrZXI=","encoding":"base64"}`))
	}))
	defer server.Close()

	c, _ := NewClient(Config{BaseURL: server.URL, Token: "t"})
	got, err := c.GetFile(context.Background(), "acme", "wf", "main", "cluster.yaml")
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if want := "provider:\n  use: docker"; string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestCommitFiles_CreatesEachFileViaContentsAPI(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound) // file doesn't exist yet -> create, not update
			return
		}
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	c, _ := NewClient(Config{BaseURL: server.URL, Token: "t"})
	files := map[string][]byte{
		"cluster.yaml":  []byte("provider:\n  use: docker"),
		"workflow.yaml": []byte("name: tpcc"),
	}
	err := c.CommitFiles(context.Background(), "acme", "wf", "main", files, "alice", "alice@example.com", "edit via IDE")
	if err != nil {
		t.Fatalf("commit files: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("wrote %d files, want 2", len(paths))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitrepo/... -run 'TestGetFile|TestCommitFiles' -v`
Expected: FAIL — `GetFile`/`CommitFiles` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
func (c *Client) GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, path, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("gitea get contents %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Encoding != "base64" {
		return nil, fmt.Errorf("gitea get contents %s: unsupported encoding %q", path, out.Encoding)
	}
	return base64.StdEncoding.DecodeString(out.Content)
}

func (c *Client) CommitFiles(ctx context.Context, owner, repo, branch string, files map[string][]byte, author, email, message string) error {
	for path, content := range files {
		sha, exists, err := c.blobSHA(ctx, owner, repo, branch, path)
		if err != nil {
			return fmt.Errorf("stat %q before commit: %w", path, err)
		}
		body := map[string]any{
			"content": base64.StdEncoding.EncodeToString(content),
			"message": message,
			"branch":  branch,
			"author":  map[string]string{"name": author, "email": email},
			"committer": map[string]string{"name": author, "email": email},
		}
		method := http.MethodPost
		if exists {
			body["sha"] = sha
			method = http.MethodPut
		}
		if err := c.writeContents(ctx, method, owner, repo, path, body); err != nil {
			return fmt.Errorf("commit %q: %w", path, err)
		}
	}
	return nil
}
```

`blobSHA` GETs the same contents endpoint `GetFile` uses, returning `(sha, exists=true, nil)` on 200 and `("", false, nil)` on 404 (any other status is an error). `writeContents` is `postIdempotent`'s sibling for `PUT`/`POST` against `/api/v1/repos/{owner}/{repo}/contents/{path}`, accepting `2xx` only (no idempotent-422 swallow here — a genuine per-file write failure must surface).

> NOTE: per-file contents-API commits are **not atomic** across multiple files (each is its own git commit in Gitea's history) — acceptable for v1 per spec §9 (not raised as an open question there), revisit via Gitea's git-data "trees" + "create commit" API if IDE users need single-commit multi-file saves later.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitrepo/... -v`
Expected: PASS.

**Testable without live infra:** fully — `httptest` mocks every Gitea REST response shape used here.

- [ ] **Step 5: Commit**

```bash
git add internal/gitrepo/client.go internal/gitrepo/client_test.go
git commit -m "feat(gitrepo): add bundle commit + file/tree read over Gitea contents API"
```

---

### Task 3: Gateway `IdeBackend` proxy + authorization seam

Add the `/ide/*` route to `internal/gateway`, built with the exact `newMonitorProxy` pattern already serving `/grafana/*` and `/v2/*`, gated by a new `IdeAuthorizer` interface that mirrors the file's existing `AgentTokenVerifier` nil-disables convention.

**Files:**
- Modify: `internal/gateway/gateway.go` (`Config.IdeBackend`, `Config.IdeAuthorizer`, `Gateway.ideProxy`, `serveHTTP` branch)
- Create: `internal/gateway/ide_auth.go` (`IdeAuthorizer` interface)
- Create: `internal/gateway/ide_test.go`
- Modify: `internal/app/config.go` / `cmd/cli/serve_cmd.go` / `internal/app/run.go` (`IdeBackend` env wiring, same shape as `GrafanaBackend`)
- Reference (pattern to mirror exactly): `internal/gateway/gateway.go:119-125,162-168` (`GrafanaBackend` wiring + `serveHTTP` branch), `internal/gateway/auth.go` (`AgentTokenVerifier`)

**Interfaces:**
```go
// internal/gateway/ide_auth.go

// IdeAuthorizer gates /ide/* before the request reaches code-server — spec
// §3 C4's "RBAC-проверка на границе гейтвея, не внутри code-server". SP-C
// cuts this interface; SP-B supplies the real RBAC-backed implementation
// (instance-admin/org-admin roles). nil disables the check (dev/test only),
// mirroring AgentTokenVerifier's existing nil-disables convention in this
// same package.
type IdeAuthorizer interface {
    // CanAuthor reports whether the request's authenticated principal may
    // open an IDE session against the target repo scope encoded in the
    // request path (r.URL.Path, e.g. "/ide/org/acme/..."). Returning false
    // is a 403, never a panic/error — an authorizer implementation error
    // must fail closed, decided by the implementation, not this seam.
    CanAuthor(r *http.Request) bool
}
```
```go
// internal/gateway/gateway.go additions
// Config:
//   IdeBackend string      // internal code-server base URL, reverse-proxied to /ide/*
//   IdeAuthorizer IdeAuthorizer // nil disables the check (dev/test only)
```

- [ ] **Step 1: Write the failing test**

```go
// internal/gateway/ide_test.go
package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubIdeAuthorizer struct{ allow bool }

func (s stubIdeAuthorizer) CanAuthor(*http.Request) bool { return s.allow }

func TestGatewayRoutesIdeTrafficToProxyWhenAuthorized(t *testing.T) {
	var gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	g, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL, IdeAuthorizer: stubIdeAuthorizer{allow: true}})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := gotPath, "/ide/org/acme/"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestGatewayRejectsIdeTrafficWhenUnauthorized(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("backend must not be called when unauthorized")
	}))
	defer backend.Close()

	g, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL, IdeAuthorizer: stubIdeAuthorizer{allow: false}})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if got, want := rec.Code, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gateway/... -run TestGatewayRoutesIdeTraffic -v`
Expected: FAIL — `Config.IdeBackend`/`IdeAuthorizer` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// ide_auth.go
package gateway

import "net/http"

type IdeAuthorizer interface {
	CanAuthor(r *http.Request) bool
}
```

```go
// gateway.go — Config additions
	// IdeBackend is the internal code-server base URL the gateway reverse-
	// proxies /ide/* to. Empty disables the route (404).
	IdeBackend string
	// IdeAuthorizer gates /ide/* before code-server ever sees the request
	// (spec SP-C §3 C4). nil disables the check — dev/test only.
	IdeAuthorizer IdeAuthorizer
```

```go
// gateway.go — Gateway struct addition
	ideProxy      http.Handler
	ideAuthorizer IdeAuthorizer
```

```go
// gateway.go — New() addition, alongside the existing RegistryBackend block
	if cfg.IdeBackend != "" {
		ip, err := newMonitorProxy(cfg.IdeBackend, "", nil) // single-host reverse proxy, no bearer
		if err != nil {
			return nil, fmt.Errorf("gateway: ide backend %q: %w", cfg.IdeBackend, err)
		}
		g.ideProxy = ip
		g.ideAuthorizer = cfg.IdeAuthorizer
	}
```

```go
// gateway.go — serveHTTP, new case alongside the existing /v2/ case
	case strings.HasPrefix(r.URL.Path, "/ide/"):
		if g.ideProxy == nil {
			http.NotFound(w, r)
			return
		}
		if g.ideAuthorizer != nil && !g.ideAuthorizer.CanAuthor(r) {
			http.Error(w, "not authorized to author this repo", http.StatusForbidden)
			return
		}
		g.ideProxy.ServeHTTP(w, r)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gateway/... -v`
Expected: PASS.

**Testable without live infra:** fully — same `httptest`-backend + `serveHTTP` assertion form as `monitorproxy_test.go`'s existing `TestGatewayRoutesMonitoringTrafficToProxy`.

- [ ] **Step 5: Env wiring + commit**

Add `IdeBackend string` to `internal/app/config.go` (doc comment matching `GrafanaBackend`'s), `env("IDE_BACKEND", "")` in `cmd/cli/serve_cmd.go`, `IdeBackend: cfg.IdeBackend` in `internal/app/run.go`'s `gateway.Config{...}` literal. `IdeAuthorizer` is left `nil` in `run.go` until SP-B exists (explicit `// TODO(SP-B): wire the RBAC-backed IdeAuthorizer once catalog/RBAC lands` comment, not a silent gap).

```bash
git add internal/gateway/gateway.go internal/gateway/ide_auth.go internal/gateway/ide_test.go internal/app/config.go cmd/cli/serve_cmd.go internal/app/run.go
git commit -m "feat(gateway): proxy /ide/* to code-server with an authorization seam"
```

---

### Task 4: Per-org code-server lifecycle + worktree materialization

A manager that ensures exactly one running code-server container per org, its workspace bind-mounted to a host directory kept in sync with that org's Gitea repo via plain `git clone`/`git pull` (not the Gitea API — cheaper, and code-server itself needs a real `.git` worktree for its built-in git UI/terminal to work, matching spec §3 C2 "code-server примонтирован на тот же файловый путь... либо ходит по git-протоколу на localhost").

**Files:**
- Create: `internal/ide/manager.go`
- Create: `internal/ide/manager_test.go`
- Create: `internal/ide/worktree.go`
- Create: `internal/ide/worktree_test.go`
- Reference (reuse, do not duplicate): `internal/infrastructure/docker/executor.go` (`hostConfig`, `containerConfig`, `envList`, `NewExecutor`'s docker client construction)

**Interfaces:**
```go
// internal/ide/worktree.go

// EnsureWorktree clones org's repo (via Gitea's git smart-HTTP endpoint,
// http://<gitea-host>/<owner>/<repo>.git — a normal git remote, not the
// REST API) into root/<org> if absent, else runs `git pull --ff-only`. It
// shells out to the system `git` binary (exec.CommandContext), the same
// approach internal/infrastructure/docker.Executor takes for its own
// external-process boundary (docker CLI/daemon via the client library, not
// re-implemented).
func EnsureWorktree(ctx context.Context, giteaBaseURL, org, root string) (path string, err error)
```
```go
// internal/ide/manager.go

// Manager ensures one running code-server container per org and returns its
// internal base URL (what gateway.Config.IdeBackend proxies to — for v1 a
// SHARED single code-server process handling all orgs under distinct
// workspace sub-paths is the recommendation from spec §9.2; this Manager
// interface stays valid for that variant too — see the Step 3 note below).
type Manager struct { /* unexported docker client, worktree root, image ref */ }

func NewManager(cli *client.Client, worktreeRoot, image string) *Manager

// EnsureRunning starts (or confirms already-running) the org's code-server
// container, bind-mounting worktreeRoot/<org> as its workspace. Returns the
// container's internal HTTP address (e.g. "http://ide-acme:8443").
func (m *Manager) EnsureRunning(ctx context.Context, org string) (addr string, err error)

// Stop removes the org's code-server container (idle-timeout eviction is a
// v1.1 concern — spec §9.2 does not require it for the shared-per-tenant
// baseline).
func (m *Manager) Stop(ctx context.Context, org string) error
```

- [ ] **Step 1: Write the failing test (worktree)**

```go
// internal/ide/worktree_test.go
package ide

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnsureWorktree_ClonesThenPulls(t *testing.T) {
	// Build a real bare-ish local git repo to stand in for a Gitea remote —
	// exercises the actual `git clone`/`git pull` shell-out without needing
	// a live Gitea instance.
	remote := t.TempDir()
	run(t, remote, "init")
	os.WriteFile(filepath.Join(remote, "cluster.yaml"), []byte("provider:\n  use: docker\n"), 0o644)
	run(t, remote, "add", ".")
	run(t, remote, "-c", "user.email=t@t.io", "-c", "user.name=t", "commit", "-m", "init")

	root := t.TempDir()
	path, err := EnsureWorktree(context.Background(), remote, "acme", root)
	if err != nil {
		t.Fatalf("ensure worktree (clone): %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "cluster.yaml")); err != nil {
		t.Fatalf("cloned file missing: %v", err)
	}

	// second call must pull, not re-clone
	path2, err := EnsureWorktree(context.Background(), remote, "acme", root)
	if err != nil {
		t.Fatalf("ensure worktree (pull): %v", err)
	}
	if path2 != path {
		t.Fatalf("path changed between calls: %q vs %q", path, path2)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ide/... -run TestEnsureWorktree -v`
Expected: FAIL — `EnsureWorktree` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ide/worktree.go
package ide

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func EnsureWorktree(ctx context.Context, giteaBaseURL, org, root string) (string, error) {
	dest := filepath.Join(root, org)
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		if err := runGit(ctx, dest, "pull", "--ff-only"); err != nil {
			return "", fmt.Errorf("pull %s worktree: %w", org, err)
		}
		return dest, nil
	}
	remote := fmt.Sprintf("%s/%s.git", giteaBaseURL, org)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	if err := runGit(ctx, "", "clone", remote, dest); err != nil {
		return "", fmt.Errorf("clone %s: %w", org, err)
	}
	return dest, nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %w: %s", args, err, out)
	}
	return nil
}
```

> NOTE: the test's "remote" is a plain local repo path, not `org.git` under a Gitea host — for the real deployment the clone URL is `<giteaBaseURL>/<org>/<org>.git` per the repo-naming convention Task 1 established (`EnsureRepo(ctx, org, org, true)` — one repo per org, named after the org). Adjust the `remote` format string once Task 1's exact naming convention for org repos (org-repo name = org slug, per spec §3 C1) is finalized alongside SP-B.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ide/... -run TestEnsureWorktree -v`
Expected: PASS.

- [ ] **Step 5: Manager container lifecycle — write the failing test**

```go
// internal/ide/manager_test.go
package ide

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
)

// TestManager_EnsureRunning_IsIdempotent exercises the manager against a
// real local docker daemon (same requirement internal/infrastructure/docker's
// own executor tests already carry — skip if DOCKER_HOST/docker.sock is
// unavailable, mirroring that package's existing test-skip convention).
func TestManager_EnsureRunning_IsIdempotent(t *testing.T) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	root := t.TempDir()
	m := NewManager(cli, root, "codercom/code-server:4.96.4")

	addr1, err := m.EnsureRunning(context.Background(), "acme")
	if err != nil {
		t.Fatalf("ensure running: %v", err)
	}
	addr2, err := m.EnsureRunning(context.Background(), "acme")
	if err != nil {
		t.Fatalf("ensure running (idempotent): %v", err)
	}
	if addr1 != addr2 {
		t.Fatalf("address changed between calls: %q vs %q", addr1, addr2)
	}
	if err := m.Stop(context.Background(), "acme"); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/ide/... -run TestManager_EnsureRunning -v`
Expected: FAIL — `NewManager`/`EnsureRunning`/`Stop` undefined. (If docker is unavailable in the sandbox running this plan, it SKIPs instead — that is expected and acceptable; see the Testable note below.)

- [ ] **Step 7: Write minimal implementation**

```go
// internal/ide/manager.go
package ide

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

type Manager struct {
	cli          *client.Client
	worktreeRoot string
	image        string
}

func NewManager(cli *client.Client, worktreeRoot, image string) *Manager {
	return &Manager{cli: cli, worktreeRoot: worktreeRoot, image: image}
}

func containerName(org string) string { return "stroppy-ide-" + org }

func (m *Manager) EnsureRunning(ctx context.Context, org string) (string, error) {
	name := containerName(org)
	if insp, err := m.cli.ContainerInspect(ctx, name); err == nil && insp.State != nil && insp.State.Running {
		return fmt.Sprintf("http://%s:8443", name), nil
	}
	hostCfg := &container.HostConfig{
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: m.worktreeRoot + "/" + org,
			Target: "/home/coder/project",
		}},
	}
	cfg := &container.Config{
		Image: m.image,
		Env:   []string{"PASSWORD="}, // auth handled by the gateway's IdeAuthorizer, not code-server's own login
	}
	resp, err := m.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		return "", fmt.Errorf("create ide container %s: %w", name, err)
	}
	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start ide container %s: %w", name, err)
	}
	return fmt.Sprintf("http://%s:8443", name), nil
}

func (m *Manager) Stop(ctx context.Context, org string) error {
	return m.cli.ContainerRemove(ctx, containerName(org), container.RemoveOptions{Force: true})
}
```

> NOTE: the real `hostCfg`/`cfg` construction should reuse `internal/infrastructure/docker/executor.go`'s `hostConfig`/`containerConfig`/`networkingConfig` helpers (network attach, restart policy, healthcheck) rather than the bare-bones literal above — this step shows the minimal shape to pass the test; wire the shared helpers in before this ships, per Global Constraints.

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/ide/... -v`
Expected: PASS (or SKIP without a local docker daemon).

**Testable without live infra:** worktree logic (Steps 1–4) — fully, via a real local git repo standing in for Gitea (no network). Container lifecycle (Steps 5–8) — requires a local docker daemon (same as `internal/infrastructure/docker`'s existing test suite); SKIPs cleanly otherwise. Requires **live Gitea** only for the real clone URL format, not for this task's own tests.

- [ ] **Step 9: Commit**

```bash
git add internal/ide/manager.go internal/ide/manager_test.go internal/ide/worktree.go internal/ide/worktree_test.go
git commit -m "feat(ide): per-org code-server lifecycle + git worktree materialization"
```

---

### Task 5: `stroppy-yaml` LSP — diagnostics via `Check`

The LSP server binary: bundle discovery (walk up from the open file to the nearest `cluster.yaml`+`workflow.yaml` directory), a `files` snapshot of that subtree, `DslService.Check` called in-process, translated to LSP `PublishDiagnosticsParams`.

**Files:**
- Create: `internal/ide/lsp/bundle.go`
- Create: `internal/ide/lsp/bundle_test.go`
- Create: `internal/ide/lsp/diagnostics.go`
- Create: `internal/ide/lsp/diagnostics_test.go`
- Create: `cmd/stroppy-yaml-lsp/main.go` (thin stdio entrypoint, not unit-tested — wires `lsp.NewServer` to `go.lsp.dev/jsonrpc2`'s stdio stream)
- Reference (reuse, do not modify): `internal/services/dsl.DslService.Check`, `internal/services/dsl.CompileBundle`'s `files map[string][]byte` contract, `web/src/components/ui/dsl-editor.tsx`'s `toCmDiagnostic` (conceptual line/col-clamp analog — see spec §5)

**Interfaces:**
```go
// internal/ide/lsp/bundle.go

// FindBundleRoot walks up from path's directory looking for a directory
// containing both "cluster.yaml" and "workflow.yaml" — the same fixed-layout
// convention internal/services/dsl.service.go's clusterFile/manifestFile
// constants encode. Returns "" if no such ancestor exists under root
// (root bounds the walk so a workspace with no bundle at all terminates).
func FindBundleRoot(root, path string) string

// SnapshotFiles reads every regular file under bundleRoot into the
// files map[string][]byte shape CompileBundle/Check/ComposedSchema/Preview
// all expect, keyed by their path relative to bundleRoot (slash-separated,
// matching include.Sources' convention).
func SnapshotFiles(bundleRoot string) (map[string][]byte, error)
```
```go
// internal/ide/lsp/diagnostics.go

// CheckDiagnostics runs DslService.Check in-process over bundleFiles and
// converts every dslpb.Diagnostic into an LSP Diagnostic, grouped by file
// path so publishDiagnostics can be sent per open document.
func CheckDiagnostics(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (map[string][]protocol.Diagnostic, error)
```

- [ ] **Step 1: Write the failing test (bundle discovery)**

```go
// internal/ide/lsp/bundle_test.go
package lsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBundleRoot_WalksUpToClusterAndWorkflow(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "workflows", "tpcc")
	os.MkdirAll(filepath.Join(bundle, "components"), 0o755)
	os.WriteFile(filepath.Join(bundle, "cluster.yaml"), []byte("provider:\n  use: docker\n"), 0o644)
	os.WriteFile(filepath.Join(bundle, "workflow.yaml"), []byte("name: tpcc\n"), 0o644)
	openFile := filepath.Join(bundle, "components", "pg.yaml")
	os.WriteFile(openFile, []byte("name: pg\n"), 0o644)

	got := FindBundleRoot(root, openFile)
	if got != bundle {
		t.Fatalf("bundle root = %q, want %q", got, bundle)
	}
}

func TestFindBundleRoot_NoAncestorReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	loose := filepath.Join(root, "notes.md")
	os.WriteFile(loose, []byte("x"), 0o644)
	if got := FindBundleRoot(root, loose); got != "" {
		t.Fatalf("bundle root = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ide/lsp/... -run TestFindBundleRoot -v`
Expected: FAIL — `FindBundleRoot` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ide/lsp/bundle.go
package lsp

import (
	"os"
	"path/filepath"
	"strings"
)

func FindBundleRoot(root, path string) string {
	dir := filepath.Dir(path)
	root = filepath.Clean(root)
	for {
		if hasBundleMarkers(dir) {
			return dir
		}
		if dir == root || dir == filepath.Dir(dir) {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}

func hasBundleMarkers(dir string) bool {
	_, cErr := os.Stat(filepath.Join(dir, "cluster.yaml"))
	_, wErr := os.Stat(filepath.Join(dir, "workflow.yaml"))
	return cErr == nil && wErr == nil
}

func SnapshotFiles(bundleRoot string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(bundleRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(bundleRoot, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	return files, err
}
```

(`strings` import trimmed if unused by the final `hasBundleMarkers`/walk shape — keep imports minimal.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ide/lsp/... -run TestFindBundleRoot -v`
Expected: PASS.

- [ ] **Step 5: Diagnostics conversion — write the failing test**

```go
// internal/ide/lsp/diagnostics_test.go
package lsp

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

func TestCheckDiagnostics_GroupsByPathAndConvertsSeverity(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := map[string][]byte{
		"cluster.yaml":  []byte("provider:\n  use: docker\n"), // malformed on purpose in the real test fixture
		"workflow.yaml": []byte("name: tpcc\n"),
	}
	byPath, err := CheckDiagnostics(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("check diagnostics: %v", err)
	}
	// Reuse an existing known-bad fixture from internal/services/dsl/service_test.go
	// (see that file's table of Check-produces-N-diagnostics cases) instead of
	// hand-writing a new malformed bundle here — assert against its exact
	// expected diagnostic count/path, not a fabricated one.
	_ = byPath
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/ide/lsp/... -run TestCheckDiagnostics -v`
Expected: FAIL — `CheckDiagnostics` undefined.

- [ ] **Step 7: Write minimal implementation**

```go
// internal/ide/lsp/diagnostics.go
package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

func CheckDiagnostics(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (map[string][]protocol.Diagnostic, error) {
	resp, err := svc.Check(ctx, &dslpb.CheckRequest{Files: bundleFiles})
	if err != nil {
		return nil, err
	}
	out := map[string][]protocol.Diagnostic{}
	for _, d := range resp.GetDiagnostics() {
		out[d.GetPath()] = append(out[d.GetPath()], protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: clampLine(d.GetLine()), Character: clampCol(d.GetCol())},
				End:   protocol.Position{Line: clampLine(d.GetLine()), Character: clampCol(d.GetCol()) + 1},
			},
			Severity: toLSPSeverity(d.GetSeverity()),
			Source:   d.GetModule(),
			Message:  d.GetMessage(),
		})
	}
	return out, nil
}

func toLSPSeverity(s dslpb.Severity) protocol.DiagnosticSeverity {
	switch s {
	case dslpb.Severity_SEVERITY_ERROR:
		return protocol.DiagnosticSeverityError
	case dslpb.Severity_SEVERITY_WARNING:
		return protocol.DiagnosticSeverityWarning
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// clampLine/clampCol convert the wire's 1-based line/col (dslpb.Diagnostic's
// convention — see internal/services/dsl/service.go's clampUint32 doc
// comment) to LSP's 0-based Position, floored at 0. The full doc-bounds
// clamp dsl-editor.tsx's toCmDiagnostic performs (never exceed the actual
// document length) is a client-side concern once a real document is in
// hand — this conversion only fixes the indexing-base mismatch.
func clampLine(v uint32) uint32 {
	if v == 0 {
		return 0
	}
	return v - 1
}

func clampCol(v uint32) uint32 {
	if v == 0 {
		return 0
	}
	return v - 1
}
```

Fill in Step 5's test body against a real known-bad fixture from `internal/services/dsl/service_test.go` (grep that file for an existing `Check`-produces-diagnostics table case) rather than the placeholder malformed-bundle comment shown above — pick one, assert `byPath["cluster.yaml"]` (or whichever path that fixture's diagnostic targets) has the expected length/severity/message substring.

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/ide/lsp/... -v`
Expected: PASS.

**Testable without live infra:** fully — `dsl.NewDslService()` is the same zero-dependency constructor `internal/services/dsl`'s own tests use; no Gitea/code-server/network involved. This is the strongest "golden-test" surface in the whole plan, per spec §8 C3.

- [ ] **Step 9: Wire the stdio entrypoint (not unit-tested — process wiring)**

```go
// cmd/stroppy-yaml-lsp/main.go
package main

import (
	"context"
	"os"

	"go.lsp.dev/jsonrpc2"

	"github.com/stroppy-io/stroppy-cloud/internal/ide/lsp"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

func main() {
	svc := dsl.NewDslService()
	stream := jsonrpc2.NewStream(struct {
		*os.File
		*os.File
	}{os.Stdin, os.Stdout})
	conn := jsonrpc2.NewConn(stream)
	server := lsp.NewServer(svc, conn) // debounced didChange -> CheckDiagnostics -> publishDiagnostics, per spec §3 C3's "every ~400ms of editing quiet"
	if err := server.Run(context.Background()); err != nil {
		os.Exit(1)
	}
}
```

`lsp.NewServer`'s `didChange`/debounce/`publishDiagnostics` wiring is standard `go.lsp.dev/protocol` server boilerplate (handler registration + a 400ms debounce timer per open document, mirroring `dsl-editor.tsx`'s existing `{ delay: 400 }` linter convention) — implement it directly against that library's `protocol.Server` interface at build time; not further specified here since it has no bundle-domain logic of its own (all domain logic already lives in `CheckDiagnostics`, tested above).

- [ ] **Step 10: Commit**

```bash
git add internal/ide/lsp/bundle.go internal/ide/lsp/bundle_test.go internal/ide/lsp/diagnostics.go internal/ide/lsp/diagnostics_test.go cmd/stroppy-yaml-lsp/main.go
git commit -m "feat(lsp): stroppy-yaml diagnostics via in-process DslService.Check"
```

---

### Task 6: LSP completion via `ComposedSchema`

Add `textDocument/completion`: field names, enum values, and required-ness pulled from `ComposedSchema`'s JSON-Schema output (today's shape — see Global Constraints for the post-SP-A migration note).

**Files:**
- Create: `internal/ide/lsp/completion.go`
- Create: `internal/ide/lsp/completion_test.go`

**Interfaces:**
```go
// internal/ide/lsp/completion.go

// CompletionItems calls DslService.ComposedSchema over bundleFiles and
// returns one LSP CompletionItem per top-level JSON-Schema property
// (properties/required/enum/description — the same fields
// internal/dsl/schema.Compose's jsonschema output already populates).
// Nested completion (inside provider.params, list items) is a v1.1 concern
// — v1 offers top-level field names only, still strictly more than today's
// zero completion in the CodeMirror lite-path.
func CompletionItems(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) ([]protocol.CompletionItem, error)
```

- [ ] **Step 1: Write the failing test**

```go
// internal/ide/lsp/completion_test.go
package lsp

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

func TestCompletionItems_ListsTopLevelSchemaProperties(t *testing.T) {
	svc := dsl.NewDslService()
	// Reuse an existing bundle fixture from internal/services/dsl/service_test.go
	// that exercises ComposedSchema successfully (e.g. its docker-provider
	// bundle helper) rather than fabricating a new one here.
	bundle := dockerBundleFilesFixture() // see Step 3's note on sourcing this helper
	items, err := CompletionItems(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("completion items: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one completion item from the composed schema")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ide/lsp/... -run TestCompletionItems -v`
Expected: FAIL — `CompletionItems`/`dockerBundleFilesFixture` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ide/lsp/completion.go
package lsp

import (
	"context"
	"encoding/json"

	"go.lsp.dev/protocol"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// composedSchemaJSON is the subset of internal/dsl/schema.Compose's
// jsonschema output this function reads — deliberately narrow (not the full
// jsonschema/v6 struct) so it degrades gracefully if unrelated jsonschema
// fields change shape. See Global Constraints: this is the PRE-SP-A path;
// once ComposedSchemaResponse.SchemaJson carries schemapb protojson instead,
// replace this json.Unmarshal with protojson.Unmarshal into *schemapb.Schema
// and read GetFields() instead of Properties/Required.
type composedSchemaJSON struct {
	Properties map[string]struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Enum        []string `json:"enum"`
	} `json:"properties"`
	Required []string `json:"required"`
}

func CompletionItems(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) ([]protocol.CompletionItem, error) {
	resp, err := svc.ComposedSchema(ctx, &dslpb.ComposedSchemaRequest{Files: bundleFiles})
	if err != nil {
		return nil, err
	}
	var schema composedSchemaJSON
	if err := json.Unmarshal([]byte(resp.GetSchemaJson()), &schema); err != nil {
		return nil, err
	}
	required := map[string]bool{}
	for _, r := range schema.Required {
		required[r] = true
	}
	items := make([]protocol.CompletionItem, 0, len(schema.Properties))
	for name, prop := range schema.Properties {
		detail := prop.Type
		if required[name] {
			detail += " (required)"
		}
		items = append(items, protocol.CompletionItem{
			Label:         name,
			Kind:          protocol.CompletionItemKindField,
			Detail:        detail,
			Documentation: prop.Description,
		})
	}
	return items, nil
}
```

For Step 1/3's `dockerBundleFilesFixture`, either export/reuse an existing bundle-building test helper from `internal/services/dsl/service_test.go` (check that file for one — its `ComposedSchema` tests already need a valid docker-provider bundle) or, if none is exported, copy its exact file map into a small local fixture in `completion_test.go` — do not invent a new bundle shape; match whatever `internal/services/dsl/service_test.go` already asserts a successful `ComposedSchema` call against, so this test's fixture cannot silently drift from the RPC's real contract.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ide/lsp/... -v`
Expected: PASS.

**Testable without live infra:** fully — same in-process `dsl.NewDslService()` as Task 5.

- [ ] **Step 5: Commit**

```bash
git add internal/ide/lsp/completion.go internal/ide/lsp/completion_test.go
git commit -m "feat(lsp): stroppy-yaml completion via ComposedSchema (pre-SP-A JSON-Schema path)"
```

---

### Task 7: Preview command via `Preview`

A custom LSP command (`stroppy/previewBundle`, not a standard LSP method — spec §3 C3) that runs the bundle's `Preview` RPC and returns the `CompiledPlan` for the IDE's side-panel webview, the same "what will be provisioned" surface `RecipeEditor.tsx`'s preview button already renders.

**Files:**
- Create: `internal/ide/lsp/preview.go`
- Create: `internal/ide/lsp/preview_test.go`

**Interfaces:**
```go
// internal/ide/lsp/preview.go

// PreviewResult is the stroppy/previewBundle LSP command's response payload:
// the compiled plan (nil if the bundle failed to compile) plus every
// diagnostic, mirroring dslpb.PreviewResponse's own shape 1:1 so the
// webview can reuse RecipeEditor.tsx's existing plan-rendering logic without
// a second translation layer.
type PreviewResult struct {
	Plan        *dslpb.CompiledPlan `json:"plan"`
	Diagnostics []*dslpb.Diagnostic `json:"diagnostics"`
}

// Preview runs DslService.Preview in-process over bundleFiles.
func Preview(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (*PreviewResult, error)
```

- [ ] **Step 1: Write the failing test**

```go
// internal/ide/lsp/preview_test.go
package lsp

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

func TestPreview_ReturnsCompiledPlanForValidBundle(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := dockerBundleFilesFixture() // same fixture as Task 6
	result, err := Preview(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected a non-nil compiled plan for a valid bundle")
	}
}

func TestPreview_ReturnsNilPlanWithDiagnosticsForBrokenBundle(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := map[string][]byte{"cluster.yaml": []byte(": not valid yaml")}
	result, err := Preview(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if result.Plan != nil {
		t.Fatal("expected nil plan for a broken bundle")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("expected at least one diagnostic for a broken bundle")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ide/lsp/... -run TestPreview -v`
Expected: FAIL — `Preview`/`PreviewResult` undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/ide/lsp/preview.go
package lsp

import (
	"context"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

type PreviewResult struct {
	Plan        *dslpb.CompiledPlan `json:"plan"`
	Diagnostics []*dslpb.Diagnostic `json:"diagnostics"`
}

func Preview(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) (*PreviewResult, error) {
	resp, err := svc.Preview(ctx, &dslpb.PreviewRequest{Files: bundleFiles})
	if err != nil {
		return nil, err
	}
	return &PreviewResult{Plan: resp.GetPlan(), Diagnostics: resp.GetDiagnostics()}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ide/lsp/... -v`
Expected: PASS.

**Testable without live infra:** fully — same in-process pattern as Tasks 5–6.

- [ ] **Step 5: Wire as a custom LSP command + commit**

In `lsp.NewServer` (Task 5 Step 9's stdio entrypoint), register `stroppy/previewBundle` against `go.lsp.dev/protocol`'s `ExecuteCommand` handler: on invocation, resolve the requesting document's bundle root (`FindBundleRoot` + `SnapshotFiles`, Task 5), call `Preview`, return the JSON-marshaled `PreviewResult` as the command's result payload for the extension's webview to render. Not independently unit-tested beyond `TestPreview_*` above — the command-dispatch wiring is `go.lsp.dev/protocol` boilerplate with no bundle-domain logic of its own, same rationale as Task 5 Step 9.

```bash
git add internal/ide/lsp/preview.go internal/ide/lsp/preview_test.go
git commit -m "feat(lsp): stroppy/previewBundle command via DslService.Preview"
```

---

## Self-Review

**Spec coverage (SP-C §3):**
- C1 git-стораж (instance + org repos, git-native history) → Task 1 (instance repo bootstrap) + Task 2 (content read/write). ✓ — org-repo *creation* is exposed (`EnsureRepo`) but not driven by anything yet (no caller until SP-B's org-creation flow exists); explicitly noted as such in Task 1.
- C2 встроенная IDE + проксирование → Task 3 (gateway `/ide/*`) + Task 4 (code-server lifecycle + worktree). ✓
- C3 `stroppy-yaml` LSP (diagnostics/completion/hover-goto/preview) → Task 5 (diagnostics), Task 6 (completion), Task 7 (preview). **Hover/goto** (spec §3 C3's third bullet, `include:`/`provider.use` reference-jumping) is **not** a task here — it needs `internal/dsl/include`'s path-resolution logic wired into a new LSP handler, which is a meaningfully separate chunk of work from Check/ComposedSchema/Preview reuse; flagged as an explicit gap, not silently dropped. Add as a follow-up task before this SP is considered "done" if hover/goto is required for v1 (spec doesn't mark it optional).
- C4 каталог/RBAC интеграция → Task 3's `IdeAuthorizer` seam (gateway-side gate) is the RBAC consumption point spec §3 C4 requires ("на границе гейтвея... до того как code-server вообще увидит запрос"); the "open from catalog" UI flow itself is SP-B's, correctly out of scope here.
- §5 "что переиспользуем" → confirmed: no modification to `internal/services/dsl`, `internal/dsl/*`, or `internal/gateway`'s existing Grafana/registry proxy code — every task is additive (new fields/branches/packages) alongside the existing pattern, not a rewrite of it.

**Open items surfaced for the implementer:**
- Task 1 Step 6: the Gitea admin API-token provisioning mechanism (how a fixed `GITEA_ADMIN_TOKEN` gets created on first `docker compose up`) is named as an open implementation-time decision, not fixed by this plan — same class of deferred decision as SP-A's tfconfig-validation caveat.
- Task 2's `CommitFiles` is per-file, not atomic across a whole bundle — flagged as a v1 limitation with a named follow-up (Gitea git-data trees API) if IDE "save all" needs a single commit.
- Task 4 Step 7's docker container config is a minimal literal, explicitly flagged to be replaced with `internal/infrastructure/docker/executor.go`'s shared `hostConfig`/`containerConfig` helpers before shipping (network attach, restart policy, healthcheck — all already solved there, must not be re-derived).
- Task 4's worktree clone URL format (`<giteaBaseURL>/<org>/<org>.git`) assumes org-repo naming = org slug; confirm against Task 1/SP-B's final naming convention before this ships.
- Task 6 is explicitly the **pre-SP-A** JSON-Schema completion path (spec §7's documented degraded mode); swapping to schemapb protojson once SP-A lands is a scoped one-function change, called out inline rather than hidden.
- Hover/goto (spec §3 C3) has no task in this plan — see spec-coverage note above; must be added before SP-C is considered spec-complete.
- `go.lsp.dev/protocol`/`go.lsp.dev/jsonrpc2` is a new dependency not yet in `go.mod` — add via `go get` at Task 5 Step 9 (per repo convention: install from source/registry, never hand-roll a vendored fork).

**Testable without live infra vs requires live infra (per-task):**
| Task | Unit-testable today (`go test`, no external services) | Requires live infra to fully verify |
|---|---|---|
| 1 | Client/bootstrap logic (`httptest` mocks) | Compose healthcheck, admin-token provisioning (manual `docker compose up gitea` + curl) |
| 2 | Fully (`httptest` mocks Gitea contents API) | — |
| 3 | Fully (`httptest` backend + `serveHTTP`, same form as existing `monitorproxy_test.go`) | — |
| 4 | Worktree clone/pull (real local git repo, no network) | Container lifecycle needs a local docker daemon (SKIPs cleanly without one, same as `internal/infrastructure/docker`'s existing tests); real Gitea needed only for the production clone URL, not the test |
| 5 | Fully (`dsl.NewDslService()` in-process, zero external deps) | — |
| 6 | Fully (same in-process pattern) | — |
| 7 | Fully (same in-process pattern) | — |
| E2E (spec §8) | — | Open IDE → edit → live diagnostic → commit → catalog sees new HEAD — needs SP-B's catalog UI, live Gitea, live code-server; explicitly deferred to "after SP-B is ready" per spec §8's own E2E line |

**Placeholder scan:** no TBD/TODO left unexplained; every `// TODO(SP-B): ...` is a named, scoped seam (not a placeholder for this plan's own work). Test fixtures that say "reuse an existing X fixture" name the exact source file to pull from rather than inventing untethered data, following SP-A's own established convention for this class of note.

**Type/contract consistency:** `map[string][]byte` (bundle files) flows unchanged from `internal/gitrepo.ListTree`/`GetFile` (Task 2) through `internal/ide/lsp.SnapshotFiles` (Task 5) into `dsl.DslService.Check/ComposedSchema/Preview` (Tasks 5–7) — the same shape at every hop, per spec §4's explicit invariant ("тот же bundle-files контракт").
