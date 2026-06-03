package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	goversion "github.com/hashicorp/go-version"
)

const (
	defaultStroppyGitHubRepo = "stroppy-io/stroppy"
	defaultStroppyVersionTTL = 10 * time.Minute
)

// GitHubStroppyVersionConfig controls GitHub release discovery.
type GitHubStroppyVersionConfig struct {
	// Repo is "owner/name"; empty defaults to stroppy-io/stroppy.
	Repo string
	// MinVersion is the inclusive minimum release version; empty disables the
	// floor. It may include or omit the leading "v".
	MinVersion string
	// Token is optional and is used only to raise GitHub API limits.
	Token string
	// TTL controls in-memory cache freshness; zero uses a sane default.
	TTL time.Duration
	// HTTPClient is injectable for tests; nil uses a client with timeout.
	HTTPClient *http.Client
	// BaseURL is injectable for tests; empty uses https://api.github.com.
	BaseURL string
}

// GitHubStroppyVersionSource lists release versions from GitHub Releases with a
// small in-memory cache. It returns normalized version strings without a leading
// "v" because the Stroppy binary resolver accepts that shape and constructs the
// GitHub artifact URL itself.
type GitHubStroppyVersionSource struct {
	client     *http.Client
	baseURL    string
	repo       string
	minVersion *goversion.Version
	token      string
	ttl        time.Duration

	mu         sync.Mutex
	cache      []string
	cacheUntil time.Time
}

func NewGitHubStroppyVersionSource(cfg GitHubStroppyVersionConfig) (*GitHubStroppyVersionSource, error) {
	repo, err := normalizeGitHubRepo(cfg.Repo)
	if err != nil {
		return nil, err
	}
	var min *goversion.Version
	if strings.TrimSpace(cfg.MinVersion) != "" {
		min, err = parseReleaseVersion(cfg.MinVersion)
		if err != nil {
			return nil, fmt.Errorf("parse stroppy minimum version: %w", err)
		}
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultStroppyVersionTTL
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &GitHubStroppyVersionSource{
		client:     client,
		baseURL:    baseURL,
		repo:       repo,
		minVersion: min,
		token:      strings.TrimSpace(cfg.Token),
		ttl:        ttl,
	}, nil
}

func (s *GitHubStroppyVersionSource) List(ctx context.Context) ([]string, error) {
	now := time.Now()
	s.mu.Lock()
	if len(s.cache) > 0 && now.Before(s.cacheUntil) {
		out := copyStrings(s.cache)
		s.mu.Unlock()
		return out, nil
	}
	stale := copyStrings(s.cache)
	s.mu.Unlock()

	versions, err := s.fetch(ctx)
	if err != nil {
		if len(stale) > 0 {
			return stale, nil
		}
		return nil, err
	}
	s.mu.Lock()
	s.cache = copyStrings(versions)
	s.cacheUntil = now.Add(s.ttl)
	s.mu.Unlock()
	return versions, nil
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func (s *GitHubStroppyVersionSource) fetch(ctx context.Context) ([]string, error) {
	seen := map[string]bool{}
	versions := make([]string, 0)
	for page := 1; page <= 5; page++ {
		releases, err := s.fetchPage(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, rel := range releases {
			if rel.Draft || rel.Prerelease {
				continue
			}
			version, parsed, err := releaseVersionFromTag(rel.TagName)
			if err != nil || parsed == nil {
				continue
			}
			if s.minVersion != nil && parsed.LessThan(s.minVersion) {
				continue
			}
			if !seen[version] {
				seen[version] = true
				versions = append(versions, version)
			}
		}
		if len(releases) < 100 {
			break
		}
	}
	sort.SliceStable(versions, func(i, j int) bool {
		vi, errI := parseReleaseVersion(versions[i])
		vj, errJ := parseReleaseVersion(versions[j])
		if errI == nil && errJ == nil {
			return vi.GreaterThan(vj)
		}
		if errI == nil {
			return true
		}
		if errJ == nil {
			return false
		}
		return versions[i] > versions[j]
	})
	return versions, nil
}

func (s *GitHubStroppyVersionSource) fetchPage(ctx context.Context, page int) ([]githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=100&page=%d", s.baseURL, s.repo, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "stroppy-cloud")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github releases %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func normalizeGitHubRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		repo = defaultStroppyGitHubRepo
	}
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.Trim(repo, "/")
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("github repo must be owner/name")
	}
	return repo, nil
}

func releaseVersionFromTag(tag string) (string, *goversion.Version, error) {
	versionText := strings.TrimSpace(tag)
	versionText = strings.TrimPrefix(versionText, "refs/tags/")
	versionText = strings.TrimPrefix(versionText, "v")
	if versionText == "" {
		return "", nil, fmt.Errorf("empty release tag")
	}
	parsed, err := parseReleaseVersion(versionText)
	if err != nil {
		return "", nil, err
	}
	return versionText, parsed, nil
}

func parseReleaseVersion(raw string) (*goversion.Version, error) {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "refs/tags/")
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nil, fmt.Errorf("empty version")
	}
	return goversion.NewVersion(v)
}

func copyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
