package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestGatewayIdeRouteNotFoundWhenBackendUnconfigured(t *testing.T) {
	g, err := New(Config{TemporalHostPort: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

func TestGatewayRefusesIdeBackendWithoutAuthorizer(t *testing.T) {
	// /ide/* proxies into a code-server holding a live worktree of a catalog
	// repo. Serving it with no authorizer would let any caller author any
	// org's (or the instance's) bundles, so New must refuse to start rather
	// than fall open. The previous test here asserted the opposite and
	// enshrined the hole as a "dev/test convention".
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("backend must never be reached without an authorizer")
	}))
	defer backend.Close()

	_, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL})
	if err == nil {
		t.Fatal("expected New to fail closed when IdeBackend is set without IdeAuthorizer")
	}
	if !strings.Contains(err.Error(), "without IdeAuthorizer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type stubIdeBackendResolver struct {
	target string
	err    error
}

func (s stubIdeBackendResolver) Backend(*http.Request) (string, error) { return s.target, s.err }

// TestGatewayRoutesIdeTrafficViaBackendResolver_WhenSet proves IdeBackends
// (Task 4's per-org multiplexing) takes priority over the static IdeBackend
// field and that different requests can be routed to different backends —
// exactly the seam T1-T3's report said Task 4 would need to add.
func TestGatewayRoutesIdeTrafficViaBackendResolver_WhenSet(t *testing.T) {
	var gotOrgAPath, gotOrgBPath string
	backendA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrgAPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backendA.Close()
	backendB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrgBPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backendB.Close()

	resolver := ideBackendByPrefix{
		"/ide/org/org-a/": backendA.URL,
		"/ide/org/org-b/": backendB.URL,
	}

	g, err := New(Config{
		TemporalHostPort: "127.0.0.1:0",
		IdeBackend:       "http://this-must-be-ignored.invalid", // IdeBackends takes priority
		IdeBackends:      resolver,
		IdeAuthorizer:    stubIdeAuthorizer{allow: true},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/org-a/file.yaml", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("org-a status = %d", rec.Code)
	}
	if gotOrgAPath != "/ide/org/org-a/file.yaml" || gotOrgBPath != "" {
		t.Fatalf("org-a request reached the wrong backend: A=%q B=%q", gotOrgAPath, gotOrgBPath)
	}

	rec2 := httptest.NewRecorder()
	g.serveHTTP(rec2, httptest.NewRequest(http.MethodGet, "/ide/org/org-b/file.yaml", nil))
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("org-b status = %d", rec2.Code)
	}
	if gotOrgBPath != "/ide/org/org-b/file.yaml" {
		t.Fatalf("org-b request never reached its own backend: got %q", gotOrgBPath)
	}
}

// TestGatewayIdeBackendResolver_ErrorIsServiceUnavailable proves a resolver
// failure (e.g. the org's code-server container could not be started) never
// falls through to any default backend — it 503s.
func TestGatewayIdeBackendResolver_ErrorIsServiceUnavailable(t *testing.T) {
	g, err := New(Config{
		TemporalHostPort: "127.0.0.1:0",
		IdeBackends:      stubIdeBackendResolver{err: errBackendUnavailable},
		IdeAuthorizer:    stubIdeAuthorizer{allow: true},
	})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}
	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

var errBackendUnavailable = errors.New("backend unavailable")

// ideBackendByPrefix is a tiny path-prefix-keyed IdeBackendResolver used
// only by this test file, standing in for internal/ide's real resolver
// (which keys by internal/ide.Scope rather than a raw path prefix).
type ideBackendByPrefix map[string]string

func (r ideBackendByPrefix) Backend(req *http.Request) (string, error) {
	for prefix, target := range r {
		if strings.HasPrefix(req.URL.Path, prefix) {
			return target, nil
		}
	}
	return "", errBackendUnavailable
}
