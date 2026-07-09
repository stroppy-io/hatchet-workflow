// Package gitrepo is a hand-rolled Gitea REST API client (SP-C's internal git
// backend, spec §9.1's "gitea sidecar" decision). It follows the same
// injectable BaseURL/HTTPClient shape as
// internal/infrastructure/adapters.GitHubStroppyVersionSource — no SDK
// dependency, plain net/http.
//
// gitrepo owns two kinds of operations:
//   - Repo lifecycle (EnsureOrg/EnsureRepo) — idempotent create-if-absent,
//     called at server startup (instance repo) and by SP-B's org-creation
//     flow (org repos), see bootstrap.go.
//   - Content (CommitFiles/GetFile/ListTree) — the same files map[string][]byte
//     shape internal/services/catalog.BundleStore and internal/services/dsl's
//     CompileBundle/Check/ComposedSchema/Preview already use. A git ref is
//     just another way to produce that map.
package gitrepo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config configures a Client.
type Config struct {
	// BaseURL is the Gitea base URL (e.g. "http://gitea:3000"); injectable for
	// tests.
	BaseURL string
	// Token is the Gitea API token, sent as "Authorization: token <Token>".
	// This credential must never be committed, baked into an image layer, or
	// cross a Temporal workflow-history boundary — it lives only in the
	// server process's environment (see internal/app.Config.GiteaToken /
	// cmd/cli/serve_cmd.go's GITEA_TOKEN env wiring), the same posture the
	// stack already uses for MONITORING_TOKEN/GF_ADMIN_PASSWORD.
	Token string
	// HTTPClient is injectable for tests; nil uses a client with a 10s
	// timeout.
	HTTPClient *http.Client
}

// Client is a minimal Gitea REST API client covering the repo-lifecycle and
// content primitives SP-C needs. It holds no state beyond connection config.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient validates cfg and returns a ready-to-use Client.
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

// EnsureOrg creates org if it does not already exist. Idempotent: Gitea's
// create-org endpoint returning 422 "already exists" is treated as success.
func (c *Client) EnsureOrg(ctx context.Context, org string) error {
	body := map[string]string{"username": org}
	return c.postIdempotent(ctx, "/api/v1/orgs", body)
}

// EnsureRepo creates owner/name if it does not already exist. owner is either
// "" (create under the token's own user — used for the instance repo, see
// bootstrap.go's InstanceRepoOwner) or an org name (caller EnsureOrg's it
// first — SP-B's org-creation flow does this for org repos). Idempotent, same
// 422-swallow convention as EnsureOrg.
func (c *Client) EnsureRepo(ctx context.Context, owner, name string, private bool) error {
	path := "/api/v1/user/repos"
	if owner != "" {
		path = fmt.Sprintf("/api/v1/orgs/%s/repos", owner)
	}
	body := map[string]any{"name": name, "private": private, "auto_init": true}
	return c.postIdempotent(ctx, path, body)
}

// postIdempotent POSTs body and treats Gitea's "already exists" response as
// success — EnsureOrg/EnsureRepo are called on every server boot / every
// org-creation flow invocation, never guarded by a prior existence check.
//
// VERIFIED against a live gitea/gitea:1.23 instance (docker-compose.yaml's
// pinned tag): repo-create-conflict comes back as 409 Conflict with body
// {"message":"The repository with the same name already exists."} — NOT the
// 422 the plan's illustrative snippet assumed (that shape matches older
// Gitea releases / other create-conflict endpoints). Both status codes are
// swallowed here since the plan's caveat ("verify against the deployed
// version") turned out to matter in practice.
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
	alreadyExists := (resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusConflict) &&
		strings.Contains(string(respBody), "already exists")
	if alreadyExists {
		return nil
	}
	return fmt.Errorf("gitea %s %s: %s: %s", http.MethodPost, path, resp.Status, strings.TrimSpace(string(respBody)))
}

// selfUser is the subset of Gitea's GET /api/v1/user response resolveOwner
// needs.
type selfUser struct {
	Login string `json:"login"`
}

// resolveOwner turns the EnsureRepo owner="" convention ("create under the
// token's own user") into a real username for the content endpoints
// (CommitFiles/GetFile/ListTree/blobSHA), which — unlike the repo-lifecycle
// endpoints — have no "self" shorthand: Gitea's contents API is always
// /repos/{owner}/{repo}/contents/{path} where {owner} must be an actual
// username or org. A non-empty owner (an org, or an explicit user) is
// returned unchanged.
func (c *Client) resolveOwner(ctx context.Context, owner string) (string, error) {
	if owner != "" {
		return owner, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("gitea whoami: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out selfUser
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Login == "" {
		return "", fmt.Errorf("gitea whoami: empty login in response")
	}
	return out.Login, nil
}

// contentsStat is the subset of Gitea's contents-API response CommitFiles
// needs to decide create-vs-update.
type contentsStat struct {
	SHA string `json:"sha"`
}

// blobSHA reports whether path already exists on branch, and if so its blob
// sha (required by Gitea's contents API to update an existing file). A 404
// means the file does not exist yet (create, not update); any other non-200
// status is a genuine error.
func (c *Client) blobSHA(ctx context.Context, owner, repo, branch, path string) (sha string, exists bool, err error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s?ref=%s", c.baseURL, owner, repo, path, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", false, fmt.Errorf("gitea stat contents %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	var out contentsStat
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", false, err
	}
	return out.SHA, true, nil
}

// writeContents POSTs (create) or PUTs (update) a single file's content via
// Gitea's contents API. It accepts only 2xx — a genuine per-file write
// failure must surface, unlike postIdempotent's 422-swallow (there is no
// "already exists" case to tolerate here: CommitFiles already resolved
// create-vs-update via blobSHA before calling this).
func (c *Client) writeContents(ctx context.Context, method, owner, repo, path string, body map[string]any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(buf))
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
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("gitea %s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// CommitFiles writes every entry of files to owner/repo on branch, one Gitea
// contents-API call per file (Gitea's contents endpoint is single-file; a
// true atomic multi-file commit needs Gitea's git-data "trees" + "create
// commit" API — deferred, not needed for the scaffold-seeding and
// single-bundle IDE-save use cases this plan targets; revisit if multi-file
// atomic saves are required later). author/email/message become each
// commit's author and message; a pre-existing file is updated (fetches its
// blob sha first, per Gitea's contents API requiring sha on update), a new
// file is created.
//
// CommitFiles is introduced here (ahead of Task 2's read-side GetFile/
// ListTree) because Task 1's instance-repo bootstrap needs a write primitive
// to seed the catalog's on-disk layout (providers/, workflows/) — see
// bootstrap.go.
func (c *Client) CommitFiles(ctx context.Context, owner, repo, branch string, files map[string][]byte, author, email, message string) error {
	owner, err := c.resolveOwner(ctx, owner)
	if err != nil {
		return fmt.Errorf("resolve owner: %w", err)
	}
	for path, content := range files {
		sha, exists, err := c.blobSHA(ctx, owner, repo, branch, path)
		if err != nil {
			return fmt.Errorf("stat %q before commit: %w", path, err)
		}
		body := map[string]any{
			"content":   base64.StdEncoding.EncodeToString(content),
			"message":   message,
			"branch":    branch,
			"author":    map[string]string{"name": author, "email": email},
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

// GetFile reads path's content at ref (branch/tag/commit sha) from
// owner/repo via Gitea's contents API. owner="" resolves to the token's own
// user (see resolveOwner), matching CommitFiles' convention.
func (c *Client) GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error) {
	owner, err := c.resolveOwner(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("resolve owner: %w", err)
	}
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

// treeEntry is the subset of Gitea's git-trees API response ListTree needs.
type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
}

// treeResponse is Gitea's GET .../git/trees/{sha}?recursive=true response
// shape.
type treeResponse struct {
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// ListTree lists every blob path under ref, recursively — the files map
// DslService.Check/ComposedSchema/Preview expect, keyed the same way
// include.Sources already keys a bundle (slash-separated logical path).
// Directory entries ("tree" type) are omitted; only file blobs are returned.
func (c *Client) ListTree(ctx context.Context, owner, repo, ref string) ([]string, error) {
	owner, err := c.resolveOwner(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("resolve owner: %w", err)
	}
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/git/trees/%s?recursive=true", c.baseURL, owner, repo, ref)
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
		return nil, fmt.Errorf("gitea list tree %s: %s: %s", ref, resp.Status, strings.TrimSpace(string(body)))
	}
	var out treeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(out.Tree))
	for _, e := range out.Tree {
		if e.Type == "blob" {
			paths = append(paths, e.Path)
		}
	}
	return paths, nil
}
