package catalog

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeGitClient is an in-memory stand-in for *internal/gitrepo.Client,
// scoped by (owner, repo, branch) exactly like a real Gitea instance would
// keep separate repos separate — used to prove GitBundleStore's isolation
// and content-addressing behavior without a live Gitea.
type fakeGitClient struct {
	mu      sync.Mutex
	commits int
	// repos[owner/repo/branch][path] = content
	repos map[string]map[string][]byte
}

func newFakeGitClient() *fakeGitClient {
	return &fakeGitClient{repos: make(map[string]map[string][]byte)}
}

func (f *fakeGitClient) key(owner, repo, branch string) string {
	return owner + "\x00" + repo + "\x00" + branch
}

func (f *fakeGitClient) CommitFiles(_ context.Context, owner, repo, branch string, files map[string][]byte, _, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits++
	k := f.key(owner, repo, branch)
	tree, ok := f.repos[k]
	if !ok {
		tree = make(map[string][]byte)
		f.repos[k] = tree
	}
	for path, content := range files {
		cp := make([]byte, len(content))
		copy(cp, content)
		tree[path] = cp
	}
	return nil
}

func (f *fakeGitClient) GetFile(_ context.Context, owner, repo, ref, path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tree, ok := f.repos[f.key(owner, repo, ref)]
	if !ok {
		return nil, errors.New("repo not found")
	}
	content, ok := tree[path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return content, nil
}

func (f *fakeGitClient) ListTree(_ context.Context, owner, repo, ref string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tree, ok := f.repos[f.key(owner, repo, ref)]
	if !ok {
		return nil, nil
	}
	paths := make([]string, 0, len(tree))
	for p := range tree {
		paths = append(paths, p)
	}
	return paths, nil
}

func TestGitBundleStore_WriteReadRoundTrip(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitBundleStore(cli, "", "instance-catalog", "main")

	files := map[string][]byte{"manifest.yaml": []byte("kind: provider\n"), "module/main.tf": []byte("# tf\n")}
	ref, err := s.Write(context.Background(), "", files)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.HasPrefix(ref, "git:") {
		t.Fatalf("ref %q missing git: scheme", ref)
	}

	got, err := s.Read(context.Background(), ref)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != len(files) {
		t.Fatalf("read back %d files, want %d", len(got), len(files))
	}
	for k, v := range files {
		if string(got[k]) != string(v) {
			t.Fatalf("file %q = %q, want %q", k, got[k], v)
		}
	}
}

func TestGitBundleStore_WriteIsIdempotentAgainstIdenticalContent(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitBundleStore(cli, "", "instance-catalog", "main")
	files := map[string][]byte{"manifest.yaml": []byte("kind: provider\n")}

	ref1, err := s.Write(context.Background(), "", files)
	if err != nil {
		t.Fatalf("write 1: %v", err)
	}
	ref2, err := s.Write(context.Background(), "", files)
	if err != nil {
		t.Fatalf("write 2: %v", err)
	}
	if ref1 != ref2 {
		t.Fatalf("ref changed between identical writes: %q vs %q", ref1, ref2)
	}
	if cli.commits != 1 {
		t.Fatalf("expected exactly 1 commit for two identical writes (idempotent), got %d", cli.commits)
	}
}

func TestGitBundleStore_DifferentContentGetsDifferentRef(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitBundleStore(cli, "", "instance-catalog", "main")

	ref1, err := s.Write(context.Background(), "", map[string][]byte{"a.yaml": []byte("v1")})
	if err != nil {
		t.Fatalf("write 1: %v", err)
	}
	ref2, err := s.Write(context.Background(), "", map[string][]byte{"a.yaml": []byte("v2")})
	if err != nil {
		t.Fatalf("write 2: %v", err)
	}
	if ref1 == ref2 {
		t.Fatalf("distinct content produced the same ref %q", ref1)
	}
}

func TestGitBundleStore_ForkAlwaysDivergesEvenWhenIdentical(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitBundleStore(cli, "", "instance-catalog", "main")
	files := map[string][]byte{"cluster.yaml": []byte("provider:\n  use: docker\n")}

	sourceRef, err := s.Write(context.Background(), "", files)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	forkedRef, err := s.Fork(context.Background(), sourceRef)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forkedRef == sourceRef {
		t.Fatalf("fork returned the same ref as its source")
	}

	// The fork's content must be byte-identical to the source (the salt
	// must never leak into the persisted bundle).
	got, err := s.Read(context.Background(), forkedRef)
	if err != nil {
		t.Fatalf("read forked: %v", err)
	}
	if string(got["cluster.yaml"]) != string(files["cluster.yaml"]) {
		t.Fatalf("forked content = %q, want %q", got["cluster.yaml"], files["cluster.yaml"])
	}

	// The ORIGINAL ref must still resolve to its own untouched content — a
	// fork must never mutate its source.
	original, err := s.Read(context.Background(), sourceRef)
	if err != nil {
		t.Fatalf("read source after fork: %v", err)
	}
	if string(original["cluster.yaml"]) != string(files["cluster.yaml"]) {
		t.Fatalf("source content changed after fork: %q", original["cluster.yaml"])
	}
}

// TestGitBundleStore_CrossRepoIsolation proves the multi-tenant invariant at
// the storage layer: two GitBundleStore instances pointed at different
// (owner, repo) pairs — the model internal/ide's per-org repos follow, one
// physical Gitea repo per tenant — cannot read each other's bundles, even
// when the underlying Gitea/fake client is shared. This is what "an org
// must not be able to read another org's ... repo" is enforced BY at the
// storage primitive: a ref minted in repo A decodes an owner/repo that
// simply has no matching data in repo B's tree, so Read fails closed
// (not-found), never silently returns cross-tenant bytes.
func TestGitBundleStore_CrossRepoIsolation(t *testing.T) {
	cli := newFakeGitClient()
	orgA := NewGitBundleStore(cli, "org-a", "org-catalog", "main")
	orgB := NewGitBundleStore(cli, "org-b", "org-catalog", "main")

	refA, err := orgA.Write(context.Background(), "", map[string][]byte{"secret.yaml": []byte("org-a-only")})
	if err != nil {
		t.Fatalf("write org a: %v", err)
	}

	if _, err := orgB.Read(context.Background(), refA); err == nil {
		t.Fatalf("org B store read org A's ref without error — cross-tenant leak")
	}
}

func TestDecodeGitSourceRef_RoundTrip(t *testing.T) {
	ref := EncodeGitSourceRef("acme", "org-catalog", "main", "deadbeef")
	owner, repo, branch, hash, err := DecodeGitSourceRef(ref)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if owner != "acme" || repo != "org-catalog" || branch != "main" || hash != "deadbeef" {
		t.Fatalf("decode = (%q,%q,%q,%q)", owner, repo, branch, hash)
	}
}

func TestDecodeGitSourceRef_EmptyOwnerAllowed(t *testing.T) {
	ref := EncodeGitSourceRef("", "instance-catalog", "main", "deadbeef")
	owner, repo, _, _, err := DecodeGitSourceRef(ref)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if owner != "" || repo != "instance-catalog" {
		t.Fatalf("decode owner/repo = (%q,%q)", owner, repo)
	}
}

func TestDecodeGitSourceRef_RejectsNonGitRef(t *testing.T) {
	if _, _, _, _, err := DecodeGitSourceRef("deadbeef"); err == nil {
		t.Fatal("expected error decoding a plain content-hash ref (not git-scheme)")
	}
}

func TestIsGitSourceRef(t *testing.T) {
	if IsGitSourceRef("deadbeef") {
		t.Fatal("plain hash ref misidentified as git ref")
	}
	if !IsGitSourceRef(EncodeGitSourceRef("", "instance-catalog", "main", "deadbeef")) {
		t.Fatal("git ref not identified as git ref")
	}
}

var _ GitClient = (*fakeGitClient)(nil)
