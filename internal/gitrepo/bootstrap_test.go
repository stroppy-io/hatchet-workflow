package gitrepo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBootstrap_CreatesInstanceRepoUnderUserRepos(t *testing.T) {
	var gotEnsureRepoPath string
	seeded := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/user/repos" && r.Method == http.MethodPost:
			gotEnsureRepoPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/api/v1/user":
			_, _ = w.Write([]byte(`{"login":"stroppy-bot"}`))
		case r.Method == http.MethodGet:
			// contents-API stat before create: nothing exists yet.
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost:
			seeded[r.URL.Path] = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := Bootstrap(context.Background(), c); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if got, want := gotEnsureRepoPath, "/api/v1/user/repos"; got != want {
		t.Fatalf("ensure-repo path = %s, want %s", got, want)
	}
	for path := range instanceLayout {
		want := "/api/v1/repos/stroppy-bot/instance-catalog/contents/" + path
		if !seeded[want] {
			t.Fatalf("layout file %q was not seeded (want POST %s), seeded=%v", path, want, seeded)
		}
	}
}

func TestBootstrap_EnsureRepoFailureSurfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := Bootstrap(context.Background(), c); err == nil {
		t.Fatal("expected error, got nil")
	}
}
