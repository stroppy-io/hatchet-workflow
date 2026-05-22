// Package stroppyrelease is a small GitHub-releases client for the
// stroppy-io/stroppy repository. It resolves the set of published stroppy
// versions (release tags) and per-asset download URLs, so the authoring backend
// can validate a workload's declared stroppy version against what actually
// exists upstream and the agent/install path can resolve a binary URL.
//
// This is the recast of the old internal/old/domain/api/server.go
// stroppyVersions / fetchStroppyCommits handlers: same public GitHub releases
// endpoint (GET https://api.github.com/repos/stroppy-io/stroppy/releases),
// same 5-10 min in-memory TTL cache so the wizard does not hammer GitHub, and
// the same graceful degradation — a network failure returns the last cached
// result (or empty) PLUS the error, never a panic. Callers (services/authoring)
// treat any error as "resolver unavailable" and fall back to the static matrix
// check.
//
// TODO(w3): a proto "list versions for the dropdown" RPC (so the wizard UI can
// populate a version picker) is a separate future step; this package gives the
// backend the data it needs without yet exposing it over the wire.
package stroppyrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	hcversion "github.com/hashicorp/go-version"
)

const (
	// releasesURL is the public GitHub releases endpoint for stroppy. per_page=100
	// fits the entire history in a single page today; pagination is not required
	// (matching the old per_page=50 behavior, just wider).
	releasesURL = "https://api.github.com/repos/stroppy-io/stroppy/releases?per_page=100"

	// defaultTTL bounds how often the client re-queries GitHub. The wizard calls
	// the resolver on every workload step; without a cache that would be one
	// GitHub API hit per keystroke-driven re-validation.
	defaultTTL = 10 * time.Minute

	// nightlyPrefix tags per-commit dev pre-releases (nightly-<short_sha>); they
	// are not "published versions" for the version dropdown, so we skip them.
	nightlyPrefix = "nightly-"
)

// release is the subset of the GitHub release object we consume.
type release struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Client resolves stroppy versions from GitHub releases, caching results in
// memory with a TTL. It is safe for concurrent use.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	ttl        time.Duration

	mu       sync.Mutex
	cached   []release
	cachedAt time.Time
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client (e.g. in tests).
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) { cl.httpClient = c }
}

// WithBaseURL overrides the releases endpoint (e.g. an httptest server in tests).
func WithBaseURL(u string) Option {
	return func(cl *Client) { cl.baseURL = u }
}

// WithTTL overrides the in-memory cache TTL.
func WithTTL(d time.Duration) Option {
	return func(cl *Client) { cl.ttl = d }
}

// New builds a Client. It honors the GITHUB_TOKEN env (sent as
// `Authorization: Bearer <token>`) to dodge GitHub's low anonymous rate limit
// when one is set; an empty token leaves requests anonymous (still works for the
// public repo).
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    releasesURL,
		token:      strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		ttl:        defaultTTL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Versions returns the published release tag names (e.g. "v4.2.1"), newest
// first. Drafts and nightly-<sha> pre-releases are excluded. On a network /
// parse failure it returns the last cached versions (possibly empty) together
// with the error — never panics.
func (c *Client) Versions(ctx context.Context) ([]string, error) {
	rels, err := c.releases(ctx)
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		if r.Draft || strings.HasPrefix(r.TagName, nightlyPrefix) || r.TagName == "" {
			continue
		}
		out = append(out, r.TagName)
	}
	sortVersionsDesc(out)
	return out, err
}

// Latest returns the newest published stable release tag (falling back to the
// newest pre-release if no stable exists). On failure it returns the latest
// from the cache (or "") plus the error.
func (c *Client) Latest(ctx context.Context) (string, error) {
	rels, err := c.releases(ctx)
	var stable, pre []string
	for _, r := range rels {
		if r.Draft || strings.HasPrefix(r.TagName, nightlyPrefix) || r.TagName == "" {
			continue
		}
		if r.Prerelease {
			pre = append(pre, r.TagName)
		} else {
			stable = append(stable, r.TagName)
		}
	}
	sortVersionsDesc(stable)
	sortVersionsDesc(pre)
	if len(stable) > 0 {
		return stable[0], err
	}
	if len(pre) > 0 {
		return pre[0], err
	}
	return "", err
}

// AssetURL returns the browser_download_url of an asset named assetName on the
// release tagged version (the agent/stroppy binary download URL). version is
// matched with or without a leading "v". Returns an error if the release or the
// asset is not found (or the resolver itself errored).
func (c *Client) AssetURL(ctx context.Context, version, assetName string) (string, error) {
	rels, err := c.releases(ctx)
	want := normalizeTag(version)
	for _, r := range rels {
		if normalizeTag(r.TagName) != want {
			continue
		}
		for _, a := range r.Assets {
			if a.Name == assetName {
				return a.BrowserDownloadURL, nil
			}
		}
		// Release found but asset is missing — report that, not a generic miss.
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("stroppyrelease: release %s has no asset %q", version, assetName)
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("stroppyrelease: release %s not found", version)
}

// releases returns the cached release list, refreshing from GitHub when the TTL
// has elapsed. On a refresh failure it returns the stale cache (possibly empty)
// plus the error so callers can degrade gracefully.
func (c *Client) releases(ctx context.Context) ([]release, error) {
	c.mu.Lock()
	if c.cached != nil && time.Since(c.cachedAt) < c.ttl {
		cached := c.cached
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	fetched, err := c.fetch(ctx)
	if err != nil {
		// Network/parse failure: return whatever we have cached + the error.
		c.mu.Lock()
		cached := c.cached
		c.mu.Unlock()
		return cached, err
	}

	c.mu.Lock()
	c.cached = fetched
	c.cachedAt = time.Now()
	c.mu.Unlock()
	return fetched, nil
}

// fetch performs the single GitHub releases GET.
func (c *Client) fetch(ctx context.Context) ([]release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("stroppyrelease: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stroppyrelease: github releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("stroppyrelease: github releases %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rels []release
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return nil, fmt.Errorf("stroppyrelease: parse releases: %w", err)
	}
	return rels, nil
}

// normalizeTag strips a leading v/V so "v4.2.1" and "4.2.1" compare equal.
func normalizeTag(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "v"), "V")
}

// sortVersionsDesc sorts release tags newest-first using semver where parseable,
// falling back to a reverse-lexical order for tags that are not valid semver
// (so the result is always deterministic).
func sortVersionsDesc(tags []string) {
	sort.SliceStable(tags, func(i, j int) bool {
		vi, ei := hcversion.NewVersion(normalizeTag(tags[i]))
		vj, ej := hcversion.NewVersion(normalizeTag(tags[j]))
		switch {
		case ei == nil && ej == nil:
			return vi.GreaterThan(vj)
		case ei == nil:
			return true // valid semver sorts before unparseable
		case ej == nil:
			return false
		default:
			return tags[i] > tags[j]
		}
	})
}
