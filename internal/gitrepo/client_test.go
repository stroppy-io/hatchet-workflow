package gitrepo

import (
	"context"
	"encoding/json"
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

func TestGetFile_DecodesBase64Content(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/repos/acme/wf/contents/cluster.yaml"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.URL.Query().Get("ref"), "main"; got != want {
			t.Fatalf("ref = %s, want %s", got, want)
		}
		_, _ = w.Write([]byte(`{"content":"cHJvdmlkZXI6CiAgdXNlOiBkb2NrZXI=","encoding":"base64"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	got, err := c.GetFile(context.Background(), "acme", "wf", "main", "cluster.yaml")
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if want := "provider:\n  use: docker"; string(got) != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestGetFile_NotFoundSurfacesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"object does not exist"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if _, err := c.GetFile(context.Background(), "acme", "wf", "main", "missing.yaml"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListTree_ReturnsOnlyBlobPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/repos/acme/wf/git/trees/main"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		if got, want := r.URL.Query().Get("recursive"), "true"; got != want {
			t.Fatalf("recursive = %s, want %s", got, want)
		}
		_, _ = w.Write([]byte(`{
			"sha": "abc",
			"tree": [
				{"path": "cluster.yaml", "type": "blob"},
				{"path": "components", "type": "tree"},
				{"path": "components/pg.yaml", "type": "blob"}
			],
			"truncated": false
		}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	got, err := c.ListTree(context.Background(), "acme", "wf", "main")
	if err != nil {
		t.Fatalf("list tree: %v", err)
	}
	want := []string{"cluster.yaml", "components/pg.yaml"}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
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

func TestForkRepo_Success202(t *testing.T) {
	// VERIFIED against a live gitea/gitea:1.23 instance: fork success is 202.
	var gotPath, gotMethod string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.ForkRepo(context.Background(), "stroppy-instance", "provider-docker-abc", "tenant-xyz", "provider-docker-abc"); err != nil {
		t.Fatalf("fork repo: %v", err)
	}
	if want := "/api/v1/repos/stroppy-instance/provider-docker-abc/forks"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotBody["organization"] != "tenant-xyz" || gotBody["name"] != "provider-docker-abc" {
		t.Fatalf("body = %v", gotBody)
	}
}

func TestForkRepo_AlreadyForked409IsNotAnError(t *testing.T) {
	// VERIFIED against a live gitea/gitea:1.23 instance: re-forking the same
	// (src, dst) pair returns 409 with a body containing "already forked".
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"repository is already forked by user [uname: tenant-xyz, repo path: stroppy-instance/provider-docker-abc, fork path: tenant-xyz/provider-docker-abc]"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.ForkRepo(context.Background(), "stroppy-instance", "provider-docker-abc", "tenant-xyz", "provider-docker-abc"); err != nil {
		t.Fatalf("fork repo should be idempotent, got: %v", err)
	}
}

func TestForkRepo_GenuineErrorSurfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := c.ForkRepo(context.Background(), "a", "b", "c", "d"); err == nil {
		t.Fatalf("expected error for 403")
	}
}

func TestRepoExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/acme/exists" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ok, err := c.RepoExists(context.Background(), "acme", "exists")
	if err != nil || !ok {
		t.Fatalf("exists = %v, %v, want true, nil", ok, err)
	}
	ok, err = c.RepoExists(context.Background(), "acme", "missing")
	if err != nil || ok {
		t.Fatalf("exists = %v, %v, want false, nil", ok, err)
	}
}

func TestLatestCommit(t *testing.T) {
	// VERIFIED against a live gitea/gitea:1.23 instance:
	// GET /repos/{owner}/{repo}/branches/{branch} -> 200, body.commit.id.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/acme/repo1/branches/main" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"name":"main","commit":{"id":"86fb7a503f0da601ee3ea46bbd4922f6bf5f7c0c"}}`))
	}))
	defer server.Close()

	c, err := NewClient(Config{BaseURL: server.URL, Token: "t"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	sha, err := c.LatestCommit(context.Background(), "acme", "repo1", "main")
	if err != nil {
		t.Fatalf("latest commit: %v", err)
	}
	if want := "86fb7a503f0da601ee3ea46bbd4922f6bf5f7c0c"; sha != want {
		t.Fatalf("sha = %q, want %q", sha, want)
	}
}
