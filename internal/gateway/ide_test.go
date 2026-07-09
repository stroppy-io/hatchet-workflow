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

func TestGatewayRoutesIdeTrafficWhenAuthorizerNil(t *testing.T) {
	var called bool
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	g, err := New(Config{TemporalHostPort: "127.0.0.1:0", IdeBackend: backend.URL})
	if err != nil {
		t.Fatalf("new gateway: %v", err)
	}

	rec := httptest.NewRecorder()
	g.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/ide/org/acme/", nil))
	if !called {
		t.Fatal("expected backend to be called when IdeAuthorizer is nil (dev/test convention)")
	}
	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}
