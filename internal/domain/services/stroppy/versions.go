package stroppy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

const cacheTTL = 5 * time.Minute

// — GitHub API response shapes ————————————————————————————————————————————

type ghAsset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []ghAsset `json:"assets"`
}

type ghCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// — Public API ———————————————————————————————————————————————————————————

// ListStroppyVersions returns a cached list of stroppy releases from GitHub.
func (s *Service) ListStroppyVersions(ctx context.Context) (*stroppypb.StroppyVersionList, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "stroppy.Service/ListStroppyVersions",
		func(ctx context.Context, _ trace.Span) (*stroppypb.StroppyVersionList, error) {
			if cached, ok, err := s.versionCache.Get(ctx, "all"); err == nil && ok {
				return cached, nil
			}

			releases, err := s.fetchReleases(ctx)
			if err != nil {
				return nil, fmt.Errorf("stroppy: fetch releases: %w", err)
			}

			versions := make([]*stroppypb.StroppyVersion, 0, len(releases))
			for _, r := range releases {
				assetURLs := make([]string, 0, len(r.Assets))
				for _, a := range r.Assets {
					assetURLs = append(assetURLs, a.BrowserDownloadURL)
				}
				versions = append(versions, &stroppypb.StroppyVersion{
					Tag:         r.TagName,
					Prerelease:  r.Prerelease,
					PublishedAt: timestamppb.New(r.PublishedAt),
					ReleaseUrl:  r.HTMLURL,
					AssetUrls:   assetURLs,
				})
			}

			list := &stroppypb.StroppyVersionList{Versions: versions}
			_ = s.versionCache.Set(ctx, "all", list, cacheTTL)
			return list, nil
		},
	)
}

// ListStroppyCommits returns a cached list of stroppy commits from GitHub.
func (s *Service) ListStroppyCommits(ctx context.Context) (*stroppypb.StroppyCommitList, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "stroppy.Service/ListStroppyCommits",
		func(ctx context.Context, _ trace.Span) (*stroppypb.StroppyCommitList, error) {
			if cached, ok, err := s.commitCache.Get(ctx, "all"); err == nil && ok {
				return cached, nil
			}

			ghCommits, err := s.fetchCommits(ctx)
			if err != nil {
				return nil, fmt.Errorf("stroppy: fetch commits: %w", err)
			}

			commits := make([]*stroppypb.StroppyCommit, 0, len(ghCommits))
			for _, c := range ghCommits {
				shortSha := c.SHA
				if len(shortSha) > 7 {
					shortSha = shortSha[:7]
				}
				commits = append(commits, &stroppypb.StroppyCommit{
					Sha:         c.SHA,
					ShortSha:    shortSha,
					Message:     c.Commit.Message,
					CommittedAt: timestamppb.New(c.Commit.Author.Date),
					Author:      c.Commit.Author.Name,
				})
			}

			list := &stroppypb.StroppyCommitList{Commits: commits}
			_ = s.commitCache.Set(ctx, "all", list, cacheTTL)
			return list, nil
		},
	)
}

// — internal helpers ——————————————————————————————————————————————————————

func (s *Service) fetchReleases(ctx context.Context) ([]ghRelease, error) {
	return fetchJSON[[]ghRelease](ctx, s.httpClient, s.releasesURL)
}

func (s *Service) fetchCommits(ctx context.Context) ([]ghCommit, error) {
	return fetchJSON[[]ghCommit](ctx, s.httpClient, s.commitsURL)
}

func fetchJSON[T any](ctx context.Context, client *http.Client, url string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return zero, fmt.Errorf("json decode: %w", err)
	}
	return result, nil
}
