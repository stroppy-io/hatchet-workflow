package fixture

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/stroppy"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/valkey"
)

// StroppyFixture wires a stroppy.Service with an in-memory valkey and a stub
// GitHub HTTP server so tests run without external dependencies.
type StroppyFixture struct {
	*IAMFixture
	Stroppy *stroppy.Service
	GH      *httptest.Server
}

// NewStroppy creates a StroppyFixture. releasesJSON and commitsJSON are the raw
// JSON bodies the stub GitHub server will return.
func NewStroppy(t *testing.T, releasesJSON, commitsJSON string) *StroppyFixture {
	t.Helper()
	iam := NewIAM(t)
	gh := httptest.NewServer(stubGitHubHandler(releasesJSON, commitsJSON))
	t.Cleanup(gh.Close)

	runner := stroppybin.New("dev", "/tmp/stroppy-test-binaries")
	vk, err := valkey.NewInMemory()
	if err != nil {
		t.Fatalf("valkey in-memory: %v", err)
	}
	t.Cleanup(vk.Close)
	svc := stroppy.New(runner, vk, gh.URL+"/releases", gh.URL+"/commits")
	return &StroppyFixture{IAMFixture: iam, Stroppy: svc, GH: gh}
}

func stubGitHubHandler(releasesJSON, commitsJSON string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/releases", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releasesJSON))
	})
	mux.HandleFunc("/commits", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(commitsJSON))
	})
	return mux
}
