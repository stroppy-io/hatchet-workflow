package stroppy

import (
	"net/http"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

// Service provides stroppy binary management and GitHub-proxy listing of
// versions and commits with a valkey JSON cache.
type Service struct {
	*tracing.Entity

	runner       *stroppybin.Runner
	httpClient   *http.Client
	versionCache *valkey.JSONCache[*stroppypb.StroppyVersionList]
	commitCache  *valkey.JSONCache[*stroppypb.StroppyCommitList]
	releasesURL  string
	commitsURL   string
}

// New constructs a Service. releasesURL and commitsURL are the GitHub API
// endpoints for releases and commits respectively.
func New(runner *stroppybin.Runner, vk *valkey.Client, releasesURL, commitsURL string) *Service {
	return &Service{
		Entity:       tracing.NewEntity("stroppy.Service"),
		runner:       runner,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		versionCache: valkey.NewJSONCache[*stroppypb.StroppyVersionList](vk, "stroppy:versions"),
		commitCache:  valkey.NewJSONCache[*stroppypb.StroppyCommitList](vk, "stroppy:commits"),
		releasesURL:  releasesURL,
		commitsURL:   commitsURL,
	}
}
