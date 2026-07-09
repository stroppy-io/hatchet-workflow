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

func TestEnsureRepo_UnderInstanceOwnerUsesUserRepos(t *testing.T) {
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
	if err := c.EnsureRepo(context.Background(), "", "instance-catalog", true); err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	if got, want := gotPath, "/api/v1/user/repos"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
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

func TestEnsureRepo_AlreadyExists409IsNotAnError(t *testing.T) {
	// Verified against a live gitea/gitea:1.23 instance: repo-create-conflict
	// is 409, not the 422 the plan's illustrative snippet assumed.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"The repository with the same name already exists."}`))
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

func TestEnsureRepo_GenuineErrorSurfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.EnsureRepo(context.Background(), "acme-org", "acme", true); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEnsureOrg_CreatesOrg(t *testing.T) {
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
	if err := c.EnsureOrg(context.Background(), "acme-org"); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	if got, want := gotPath, "/api/v1/orgs"; got != want {
		t.Fatalf("path = %s, want %s", got, want)
	}
}

func TestNewClient_RejectsEmptyBaseURL(t *testing.T) {
	if _, err := NewClient(Config{Token: "t"}); err == nil {
		t.Fatal("expected error for empty BaseURL, got nil")
	}
}

func TestCommitFiles_CreatesEachFileViaContentsAPI(t *testing.T) {
	var creates []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound) // file doesn't exist yet -> create, not update
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST for a new file, got %s", r.Method)
		}
		creates = append(creates, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	files := map[string][]byte{
		"providers/.gitkeep": {},
		"workflows/.gitkeep": {},
	}
	if err := c.CommitFiles(context.Background(), "acme-org", "instance-catalog", "main", files, "stroppy-bootstrap", "bootstrap@stroppy.local", "seed catalog layout"); err != nil {
		t.Fatalf("commit files: %v", err)
	}
	if len(creates) != 2 {
		t.Fatalf("wrote %d files, want 2", len(creates))
	}
}

func TestCommitFiles_UpdatesExistingFileViaPUT(t *testing.T) {
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sha":"abc123"}`))
			return
		}
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	files := map[string][]byte{"providers/docker/manifest.yaml": []byte("name: docker\n")}
	if err := c.CommitFiles(context.Background(), "acme-org", "instance-catalog", "main", files, "a", "a@a.io", "update"); err != nil {
		t.Fatalf("commit files: %v", err)
	}
	if got, want := gotMethod, http.MethodPut; got != want {
		t.Fatalf("method = %s, want %s", got, want)
	}
}

func TestCommitFiles_EmptyOwnerResolvesToTokenOwnerViaWhoami(t *testing.T) {
	var gotContentsPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/user":
			_, _ = w.Write([]byte(`{"login":"stroppy-bot"}`))
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusNotFound)
		default:
			gotContentsPath = r.URL.Path
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.CommitFiles(context.Background(), "", "instance-catalog", "main", map[string][]byte{"README.md": []byte("x")}, "bot", "bot@x.io", "seed"); err != nil {
		t.Fatalf("commit files: %v", err)
	}
	if want := "/api/v1/repos/stroppy-bot/instance-catalog/contents/README.md"; gotContentsPath != want {
		t.Fatalf("contents path = %q, want %q", gotContentsPath, want)
	}
}
