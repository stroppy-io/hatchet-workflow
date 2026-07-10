package catalog

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeGitClient is an in-memory stand-in for *internal/gitrepo.Client,
// scoped by (owner, repo) exactly like a real Gitea instance would keep
// separate repos separate — used to prove GitEntryBundleStore's
// repo-per-entry and isolation behavior without a live Gitea. Every commit
// bumps a per-repo counter used as its "sha" (LatestCommit reads the
// counter's current value), so distinct commits are trivially
// distinguishable in assertions without hashing real git trees.
type fakeGitClient struct {
	mu    sync.Mutex
	repos map[string]*fakeRepo
}

type fakeRepo struct {
	owner, name string
	orgEnsured  bool
	commitN     int
	// tree[sha][path] = content — one snapshot per commit, so Read at an
	// older sha still sees that commit's exact tree even after a later
	// commit changes files (mirrors real git's immutability).
	tree map[string]map[string][]byte
	fork string // non-"" => the (owner/name) this repo was forked from.
}

func newFakeGitClient() *fakeGitClient {
	return &fakeGitClient{repos: make(map[string]*fakeRepo)}
}

func (f *fakeGitClient) key(owner, repo string) string { return owner + "/" + repo }

func (f *fakeGitClient) EnsureOrg(_ context.Context, _ string) error { return nil }

func (f *fakeGitClient) EnsureRepo(_ context.Context, owner, name string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := f.key(owner, name)
	if _, ok := f.repos[k]; !ok {
		f.repos[k] = &fakeRepo{owner: owner, name: name, tree: make(map[string]map[string][]byte)}
	}
	return nil
}

func (f *fakeGitClient) RepoExists(_ context.Context, owner, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.repos[f.key(owner, name)]
	return ok, nil
}

func (f *fakeGitClient) ForkRepo(_ context.Context, srcOwner, srcRepo, dstOwner, dstName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	srcKey := f.key(srcOwner, srcRepo)
	if _, ok := f.repos[srcKey]; !ok {
		return errors.New("fake: fork source repo not found")
	}
	dstKey := f.key(dstOwner, dstName)
	if _, ok := f.repos[dstKey]; ok {
		return errors.New("already forked")
	}
	f.repos[dstKey] = &fakeRepo{owner: dstOwner, name: dstName, tree: make(map[string]map[string][]byte), fork: srcKey}
	return nil
}

func (f *fakeGitClient) CommitFiles(_ context.Context, owner, repo, branch string, files map[string][]byte, _, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[f.key(owner, repo)]
	if !ok {
		return errors.New("fake: repo not found, EnsureRepo not called")
	}
	r.commitN++
	sha := f.shaFor(r, r.commitN)
	snapshot := make(map[string][]byte)
	if r.commitN > 1 {
		for p, c := range r.tree[f.shaFor(r, r.commitN-1)] {
			snapshot[p] = c
		}
	}
	for path, content := range files {
		cp := make([]byte, len(content))
		copy(cp, content)
		snapshot[path] = cp
	}
	r.tree[sha] = snapshot
	// Also alias the branch name to the same snapshot — a real Gitea
	// contents/tree API accepts either a branch name or a commit sha as
	// "ref"; the legacy monorepo reader (GitMonorepoBundleStore) always
	// reads by branch, the repo-per-entry store always reads by the sha
	// LatestCommit returned, so this fake must satisfy both.
	r.tree[branch] = snapshot
	return nil
}

func (f *fakeGitClient) shaFor(r *fakeRepo, n int) string {
	return "sha-" + r.owner + "-" + r.name + "-" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func (f *fakeGitClient) LatestCommit(_ context.Context, owner, repo, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[f.key(owner, repo)]
	if !ok || r.commitN == 0 {
		return "", errors.New("fake: no commits yet")
	}
	return f.shaFor(r, r.commitN), nil
}

func (f *fakeGitClient) GetFile(_ context.Context, owner, repo, ref, path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[f.key(owner, repo)]
	if !ok {
		return nil, errors.New("repo not found")
	}
	snapshot, ok := r.tree[ref]
	if !ok {
		return nil, errors.New("ref not found")
	}
	content, ok := snapshot[path]
	if !ok {
		return nil, errors.New("file not found")
	}
	return content, nil
}

func (f *fakeGitClient) ListTree(_ context.Context, owner, repo, ref string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[f.key(owner, repo)]
	if !ok {
		return nil, nil
	}
	snapshot, ok := r.tree[ref]
	if !ok {
		return nil, nil
	}
	paths := make([]string, 0, len(snapshot))
	for p := range snapshot {
		paths = append(paths, p)
	}
	return paths, nil
}

var _ GitClient = (*fakeGitClient)(nil)

func instanceProvider(slug string) BundleIdentity {
	return BundleIdentity{IsInstanceLevel: true, KindStr: "provider", Slug: slug}
}

func orgProvider(tenantID, slug string) BundleIdentity {
	return BundleIdentity{TenantID: tenantID, KindStr: "provider", Slug: slug}
}

func TestGitEntryBundleStore_WriteReadRoundTrip(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitEntryBundleStore(cli)

	files := map[string][]byte{"manifest.yaml": []byte("kind: provider\n"), "module/main.tf": []byte("# tf\n")}
	ref, err := s.Write(context.Background(), instanceProvider("docker"), "", files)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.HasPrefix(ref, "git:") || !strings.Contains(ref, "@") {
		t.Fatalf("ref %q does not look like a commit-pinned ref", ref)
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

func TestGitEntryBundleStore_SameItemTwoVersionsShareOneRepoDifferentCommits(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitEntryBundleStore(cli)
	id := instanceProvider("docker")

	ref1, err := s.Write(context.Background(), id, "", map[string][]byte{"a.yaml": []byte("v1")})
	if err != nil {
		t.Fatalf("write 1: %v", err)
	}
	ref2, err := s.Write(context.Background(), id, "", map[string][]byte{"a.yaml": []byte("v2")})
	if err != nil {
		t.Fatalf("write 2: %v", err)
	}
	if ref1 == ref2 {
		t.Fatalf("two distinct versions produced the same ref %q", ref1)
	}
	o1, r1, _, err := DecodeGitCommitRef(ref1)
	if err != nil {
		t.Fatalf("decode ref1: %v", err)
	}
	o2, r2, _, err := DecodeGitCommitRef(ref2)
	if err != nil {
		t.Fatalf("decode ref2: %v", err)
	}
	if o1 != o2 || r1 != r2 {
		t.Fatalf("two versions of the same item landed in different repos: %s/%s vs %s/%s", o1, r1, o2, r2)
	}
	// The old version must still resolve to its own untouched content.
	got1, err := s.Read(context.Background(), ref1)
	if err != nil {
		t.Fatalf("read ref1 after ref2 written: %v", err)
	}
	if string(got1["a.yaml"]) != "v1" {
		t.Fatalf("ref1 content = %q, want v1", got1["a.yaml"])
	}
}

func TestGitEntryBundleStore_DifferentSlugsGetDifferentRepos(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitEntryBundleStore(cli)

	refDocker, err := s.Write(context.Background(), instanceProvider("docker"), "", map[string][]byte{"manifest.yaml": []byte("x")})
	if err != nil {
		t.Fatalf("write docker: %v", err)
	}
	refYandex, err := s.Write(context.Background(), instanceProvider("yandex"), "", map[string][]byte{"manifest.yaml": []byte("x")})
	if err != nil {
		t.Fatalf("write yandex: %v", err)
	}
	_, repoDocker, _, _ := DecodeGitCommitRef(refDocker)
	_, repoYandex, _, _ := DecodeGitCommitRef(refYandex)
	if repoDocker == repoYandex {
		t.Fatalf("distinct slugs landed in the same repo %q", repoDocker)
	}
}

func TestGitEntryBundleStore_Fork_RealForkThenContentSync(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitEntryBundleStore(cli)
	files := map[string][]byte{"manifest.yaml": []byte("kind: provider\n")}

	srcRef, err := s.Write(context.Background(), instanceProvider("docker"), "", files)
	if err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstID := orgProvider("tenant-1", "docker")
	forkedRef, err := s.Fork(context.Background(), dstID, srcRef)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forkedRef == srcRef {
		t.Fatalf("fork returned the same ref as its source")
	}
	dstOwner, dstRepo, _, _ := DecodeGitCommitRef(forkedRef)
	srcOwner, srcRepo, _, _ := DecodeGitCommitRef(srcRef)
	if dstOwner == srcOwner && dstRepo == srcRepo {
		t.Fatalf("fork landed in the SAME repo as its source")
	}
	// The fake client's ForkRepo must have been called to establish real
	// fork ancestry.
	cli.mu.Lock()
	fork := cli.repos[cli.key(dstOwner, dstRepo)].fork
	cli.mu.Unlock()
	if fork != cli.key(srcOwner, srcRepo) {
		t.Fatalf("forked repo's fork-parent = %q, want %q", fork, cli.key(srcOwner, srcRepo))
	}

	got, err := s.Read(context.Background(), forkedRef)
	if err != nil {
		t.Fatalf("read forked: %v", err)
	}
	if string(got["manifest.yaml"]) != string(files["manifest.yaml"]) {
		t.Fatalf("forked content = %q, want %q", got["manifest.yaml"], files["manifest.yaml"])
	}

	original, err := s.Read(context.Background(), srcRef)
	if err != nil {
		t.Fatalf("read source after fork: %v", err)
	}
	if string(original["manifest.yaml"]) != string(files["manifest.yaml"]) {
		t.Fatalf("source content changed after fork: %q", original["manifest.yaml"])
	}
}

func TestGitEntryBundleStore_Fork_IdempotentAgainstExistingDestRepo(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitEntryBundleStore(cli)
	files := map[string][]byte{"manifest.yaml": []byte("v1")}

	srcRef, err := s.Write(context.Background(), instanceProvider("docker"), "", files)
	if err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstID := orgProvider("tenant-1", "docker")
	if _, err := s.Fork(context.Background(), dstID, srcRef); err != nil {
		t.Fatalf("first fork: %v", err)
	}
	// A second fork (e.g. a re-fork after another edit at the source) must
	// not fail even though the destination repo already exists.
	if _, err := s.Fork(context.Background(), dstID, srcRef); err != nil {
		t.Fatalf("second fork should be idempotent, got: %v", err)
	}
}

// TestGitEntryBundleStore_CrossTenantRepoIsolation proves two tenants'
// entries of the SAME (kind, slug) never share a repo — the naming scheme
// derives the owning org from tenantID, so org-a and org-b's "docker"
// provider each get their own physical repo even though EntryRepoName's
// repo-name component (kind+slug) is identical.
func TestGitEntryBundleStore_CrossTenantRepoIsolation(t *testing.T) {
	cli := newFakeGitClient()
	s := NewGitEntryBundleStore(cli)

	refA, err := s.Write(context.Background(), orgProvider("tenant-a", "docker"), "", map[string][]byte{"secret.yaml": []byte("tenant-a-only")})
	if err != nil {
		t.Fatalf("write tenant a: %v", err)
	}
	ownerA, repoA, _, _ := DecodeGitCommitRef(refA)

	refB, err := s.Write(context.Background(), orgProvider("tenant-b", "docker"), "", map[string][]byte{"secret.yaml": []byte("tenant-b-only")})
	if err != nil {
		t.Fatalf("write tenant b: %v", err)
	}
	ownerB, repoB, _, _ := DecodeGitCommitRef(refB)

	if ownerA == ownerB {
		t.Fatalf("tenant a and tenant b resolved to the SAME gitea org %q", ownerA)
	}
	_ = repoA
	_ = repoB
}

func TestDecodeGitCommitRef_RoundTrip(t *testing.T) {
	ref := EncodeGitCommitRef("tenant-abc", "provider-docker-1234abcd", "deadbeef")
	owner, repo, sha, err := DecodeGitCommitRef(ref)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if owner != "tenant-abc" || repo != "provider-docker-1234abcd" || sha != "deadbeef" {
		t.Fatalf("decode = (%q,%q,%q)", owner, repo, sha)
	}
}

func TestDecodeGitCommitRef_RejectsLegacyShapes(t *testing.T) {
	if _, _, _, err := DecodeGitCommitRef("deadbeef"); err == nil {
		t.Fatal("expected error decoding a plain content-hash ref")
	}
	if _, _, _, err := DecodeGitCommitRef("git:acme/instance-catalog/main/deadbeef"); err == nil {
		t.Fatal("expected error decoding a legacy monorepo-shaped ref (no '@')")
	}
}

func TestIsGitCommitRef(t *testing.T) {
	if IsGitCommitRef("deadbeef") {
		t.Fatal("plain hash ref misidentified as a git commit ref")
	}
	if IsGitCommitRef("git:acme/instance-catalog/main/deadbeef") {
		t.Fatal("legacy monorepo ref misidentified as a git commit ref")
	}
	if !IsGitCommitRef(EncodeGitCommitRef("acme", "provider-docker-1234abcd", "deadbeef")) {
		t.Fatal("commit-pinned ref not identified as such")
	}
}

func TestIsLegacyMonorepoRef(t *testing.T) {
	if !IsLegacyMonorepoRef("git:acme/instance-catalog/main/deadbeef") {
		t.Fatal("legacy monorepo ref not identified as such")
	}
	if IsLegacyMonorepoRef("deadbeef") {
		t.Fatal("bare ref misidentified as legacy monorepo ref")
	}
	if IsLegacyMonorepoRef(EncodeGitCommitRef("acme", "repo", "sha")) {
		t.Fatal("commit-pinned ref misidentified as legacy monorepo ref")
	}
}

func TestGitMonorepoBundleStore_ReadsLegacyShape(t *testing.T) {
	cli := newFakeGitClient()
	// Seed the legacy monorepo layout directly via CommitFiles at the fixed
	// (owner="", instance-catalog, main) location the old GitBundleStore
	// always wrote to.
	if err := cli.EnsureRepo(context.Background(), "", "instance-catalog", true); err != nil {
		t.Fatalf("ensure legacy repo: %v", err)
	}
	if err := cli.CommitFiles(context.Background(), "", "instance-catalog", "main",
		map[string][]byte{"bundles/de/deadbeef/manifest.yaml": []byte("legacy content")}, "x", "x@x", "seed"); err != nil {
		t.Fatalf("seed legacy layout: %v", err)
	}
	legacy := NewGitMonorepoBundleStore(cli)
	files, err := legacy.Read(context.Background(), "git:/instance-catalog/main/deadbeef")
	if err != nil {
		t.Fatalf("read legacy ref: %v", err)
	}
	if string(files["manifest.yaml"]) != "legacy content" {
		t.Fatalf("legacy content = %q", files["manifest.yaml"])
	}
}
