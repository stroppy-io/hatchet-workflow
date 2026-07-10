package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/gitrepo"
)

// GitClient is the subset of *internal/gitrepo.Client GitEntryBundleStore
// (and the legacy GitBundleStore reader below) need. Declaring it here
// (rather than importing *gitrepo.Client directly) keeps this file trivially
// testable with a fake — *gitrepo.Client already satisfies it structurally.
type GitClient interface {
	EnsureOrg(ctx context.Context, org string) error
	EnsureRepo(ctx context.Context, owner, name string, private bool) error
	CommitFiles(ctx context.Context, owner, repo, branch string, files map[string][]byte, author, email, message string) error
	GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error)
	ListTree(ctx context.Context, owner, repo, ref string) ([]string, error)
	RepoExists(ctx context.Context, owner, repo string) (bool, error)
	ForkRepo(ctx context.Context, srcOwner, srcRepo, dstOwner, dstName string) error
	LatestCommit(ctx context.Context, owner, repo, branch string) (string, error)
}

// gitCommitRefPrefix distinguishes a repo-per-entry, commit-pinned ref
// (GitEntryBundleStore's format, "git:<owner>/<repo>@<sha>") from every
// other ref shape a catalog_entries row's source_ref might still carry: a
// bare content hash (FSBundleStore/MemoryBundleStore, pre-git), or the
// retired single-monorepo "git:<owner>/<repo>/<branch>/<hash>" shape the
// original GitBundleStore (now kept only as a legacy reader, below) minted.
const gitCommitRefPrefix = "git:"

// EncodeGitCommitRef builds the source_ref string for a commit-pinned,
// repo-per-entry bundle: owner and repo identify the entry's own Gitea repo
// (see gitrepo.OwnerFor/EntryRepoName), sha is the immutable commit that
// version's files were committed as. Self-describing: every field is
// recoverable from the string alone via DecodeGitCommitRef.
func EncodeGitCommitRef(owner, repo, sha string) string {
	return fmt.Sprintf("%s%s/%s@%s", gitCommitRefPrefix, owner, repo, sha)
}

// DecodeGitCommitRef parses a ref minted by EncodeGitCommitRef. Every field
// must be non-empty; owner/repo must not themselves contain "@" or a second
// "/"-delimited path segment (a malformed or hostile ref is rejected rather
// than guessed at).
func DecodeGitCommitRef(ref string) (owner, repo, sha string, err error) {
	if !strings.HasPrefix(ref, gitCommitRefPrefix) {
		return "", "", "", fmt.Errorf("git commit ref %q: missing %q scheme", ref, gitCommitRefPrefix)
	}
	rest := strings.TrimPrefix(ref, gitCommitRefPrefix)
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return "", "", "", fmt.Errorf("git commit ref %q: expected owner/repo@sha", ref)
	}
	ownerRepo, sha := rest[:at], rest[at+1:]
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("git commit ref %q: expected owner/repo@sha", ref)
	}
	owner, repo = parts[0], parts[1]
	if owner == "" || repo == "" || sha == "" {
		return "", "", "", fmt.Errorf("git commit ref %q: owner, repo and sha must be non-empty", ref)
	}
	return owner, repo, sha, nil
}

// IsGitCommitRef reports whether ref was minted by GitEntryBundleStore (the
// active, repo-per-entry shape) — as opposed to a legacy bare content hash
// or the retired monorepo "git:owner/repo/branch/hash" shape, both of which
// healSourceRef (catalog.go) migrates on first read.
func IsGitCommitRef(ref string) bool {
	_, _, _, err := DecodeGitCommitRef(ref)
	return err == nil
}

// gitEntryAuthorName/Email stamp every commit GitEntryBundleStore makes —
// distinct from any real end-user, so bot-authored commits (bundle writes,
// fork-content-sync) read as infrastructure in `git log`, matching
// gitrepo.Bootstrap's bootstrapAuthorName convention.
const (
	gitEntryAuthorName  = "stroppy-bot"
	gitEntryAuthorEmail = "bot@stroppy.local"
)

// gitEntryBranch is the fixed default branch every entry repo this store
// creates uses — mirrors gitrepo.InstanceRepoBranch, generalized to every
// per-entry repo (not just the retired singleton instance-catalog repo).
const gitEntryBranch = "main"

// GitEntryBundleStore is the repo-per-entry BundleStore: every catalog item
// (one BundleIdentity — level/tenant/kind/slug) gets its OWN Gitea repo, and
// every version of that item is a commit on that repo's default branch —
// exactly the product decision this task implements ("каждый провайдер или
// воркфлоу подготовленый это отдельный репозиторий полностью"). This
// replaces the old single-monorepo GitBundleStore (kept below, renamed
// GitMonorepoBundleStore, purely as a read-only LegacyBundles source for
// migrating rows written before this redesign).
//
// # Repo naming and isolation
//
// gitrepo.OwnerFor(id.IsInstanceLevel, id.TenantID) picks the Gitea org
// (gitrepo.InstanceOrg, fixed, for LEVEL_INSTANCE; gitrepo.TenantOrg
// (tenantID), one org per tenant, for LEVEL_ORG) and
// gitrepo.EntryRepoName(id.KindStr, id.Slug) picks the repo name within it —
// both pure, sanitizing functions of their inputs (see their own docs for
// how a hostile slug is contained: sanitized + hashed, never able to escape
// its own org or collide with another entry's repo name).
//
// # Multi-tenant read trust model
//
// This store, like the old single-repo GitBundleStore before it, is wired
// as ONE shared instance across every tenant (catalog.Deps.Bundles). Unlike
// the old store, Read here does NOT (cannot, now that there are
// unboundedly many repos) pin itself to one owner/repo — it decodes
// whatever ref it is given and reads that repo. This is safe because a ref
// is never accepted from a client directly: every ref this package's
// service.go hands to Read came from a catalog_entries row already fetched
// through CatalogEntryRepo.Get(level, tenantID, id), which is itself scoped
// to the caller's authorized tenant. BundleStore remains, as documented in
// bundlestore.go, a dumb bytes-behind-a-ref cache — cross-tenant
// authorization is enforced by the DB row scoping one layer up in
// service.go, exactly as it always has been. (The one place a physical
// repo IS reachable directly, bypassing that row-scoping — the IDE's
// code-server worktrees — enforces isolation itself via
// internal/ide.Scope/Authorizer; see that package's docs.)
type GitEntryBundleStore struct {
	client GitClient
}

var _ BundleStore = (*GitEntryBundleStore)(nil)

// NewGitEntryBundleStore returns a GitEntryBundleStore backed by client.
func NewGitEntryBundleStore(client GitClient) *GitEntryBundleStore {
	return &GitEntryBundleStore{client: client}
}

// repoFor resolves id's own (owner, repo) — the one Gitea repo every
// version of this catalog item lives in.
func repoFor(id BundleIdentity) (owner, repo string) {
	return gitrepo.OwnerFor(id.IsInstanceLevel, id.TenantID), gitrepo.EntryRepoName(id.KindStr, id.Slug)
}

// Write ensures id's own repo exists (creating its org/repo if this is the
// item's first version) and commits files onto its default branch as a new
// commit — NOT a content-addressed no-op the way the old monorepo store's
// Write was: every version is meant to be its own real commit in the
// entry's history (spec: "История, ветки, диффы — нативные git-примитивы"),
// so a byte-identical re-save still produces a (empty-diff, but real) commit
// rather than being suppressed. The returned ref pins the resulting commit.
func (s *GitEntryBundleStore) Write(ctx context.Context, id BundleIdentity, _ string, files map[string][]byte) (string, error) {
	owner, repo := repoFor(id)
	if err := s.ensureEntryRepo(ctx, owner, repo); err != nil {
		return "", err
	}
	msg := fmt.Sprintf("chore: store %s catalog bundle", id.KindStr)
	if err := s.client.CommitFiles(ctx, owner, repo, gitEntryBranch, files, gitEntryAuthorName, gitEntryAuthorEmail, msg); err != nil {
		return "", fmt.Errorf("git entry bundle store: commit %s/%s: %w", owner, repo, err)
	}
	sha, err := s.client.LatestCommit(ctx, owner, repo, gitEntryBranch)
	if err != nil {
		return "", fmt.Errorf("git entry bundle store: latest commit %s/%s: %w", owner, repo, err)
	}
	return EncodeGitCommitRef(owner, repo, sha), nil
}

// ensureEntryRepo idempotently creates owner (an org) and owner/repo,
// private, auto-initialized — the same EnsureOrg+EnsureRepo pair
// internal/ide.Manager.giteaOwnerRepo already calls for an org's IDE
// worktree repo, called here too since a catalog write can be this item's
// very first version.
func (s *GitEntryBundleStore) ensureEntryRepo(ctx context.Context, owner, repo string) error {
	if err := s.client.EnsureOrg(ctx, owner); err != nil {
		return fmt.Errorf("git entry bundle store: ensure org %q: %w", owner, err)
	}
	if err := s.client.EnsureRepo(ctx, owner, repo, true); err != nil {
		return fmt.Errorf("git entry bundle store: ensure repo %s/%s: %w", owner, repo, err)
	}
	return nil
}

// Read decodes ref (owner/repo@sha) and reads every file at that commit back
// into a map keyed by its repo-relative path — files live at the repo
// ROOT now (no bundles/<hash>/ sharding prefix: that was the monorepo
// store's content-addressing scheme; a repo-per-entry store has no sibling
// bundles to shard away from).
func (s *GitEntryBundleStore) Read(ctx context.Context, ref string) (map[string][]byte, error) {
	owner, repo, sha, err := DecodeGitCommitRef(ref)
	if err != nil {
		return nil, fmt.Errorf("git entry bundle store: %w", err)
	}
	paths, err := s.client.ListTree(ctx, owner, repo, sha)
	if err != nil {
		return nil, fmt.Errorf("git entry bundle store: list %q: %w", ref, err)
	}
	files := make(map[string][]byte, len(paths))
	for _, p := range paths {
		content, err := s.client.GetFile(ctx, owner, repo, sha, p)
		if err != nil {
			return nil, fmt.Errorf("git entry bundle store: read %q at %q: %w", p, ref, err)
		}
		files[p] = content
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("git commit ref %q not found (empty tree)", ref)
	}
	return files, nil
}

// Fork materializes id's OWN repo as a real Gitea fork of sourceRef's repo
// (satisfying the product decision's ORIGIN_FORKED semantics: "реальный
// форк в гитею тенанта"), then commits sourceRef's exact files onto it —
// the explicit commit (rather than trusting Gitea's fork to have copied the
// right branch state) guarantees the fork's tip is exactly the pinned
// version being forked, independent of whatever Gitea forked from
// (typically the source repo's CURRENT default-branch tip, which may have
// moved past sourceRef's commit by the time this fork happens).
//
// Idempotent: if id's repo already exists (a previous fork, or a second
// edit after the first fork), the real-fork step is skipped (RepoExists
// check) and only the content-sync commit runs — re-forking the same (src,
// dst) pair is itself idempotent at the Gitea API level too (see
// gitrepo.Client.ForkRepo's doc: 409 "already forked" is swallowed), so this
// is doubly safe under a race.
func (s *GitEntryBundleStore) Fork(ctx context.Context, id BundleIdentity, sourceRef string) (string, error) {
	srcOwner, srcRepo, _, err := DecodeGitCommitRef(sourceRef)
	if err != nil {
		return "", fmt.Errorf("git entry bundle store: fork: %w", err)
	}
	files, err := s.Read(ctx, sourceRef)
	if err != nil {
		return "", fmt.Errorf("git entry bundle store: fork: read source: %w", err)
	}
	dstOwner, dstRepo := repoFor(id)
	if err := s.client.EnsureOrg(ctx, dstOwner); err != nil {
		return "", fmt.Errorf("git entry bundle store: fork: ensure dest org %q: %w", dstOwner, err)
	}
	exists, err := s.client.RepoExists(ctx, dstOwner, dstRepo)
	if err != nil {
		return "", fmt.Errorf("git entry bundle store: fork: check dest repo %s/%s: %w", dstOwner, dstRepo, err)
	}
	if !exists {
		if err := s.client.ForkRepo(ctx, srcOwner, srcRepo, dstOwner, dstRepo); err != nil {
			return "", fmt.Errorf("git entry bundle store: fork %s/%s -> %s/%s: %w", srcOwner, srcRepo, dstOwner, dstRepo, err)
		}
	}
	msg := fmt.Sprintf("chore: fork %s catalog bundle from %s/%s", id.KindStr, srcOwner, srcRepo)
	// A fork-salt marker file is unnecessary here (unlike the old
	// content-addressed stores' forkRef trick): a real git fork is a
	// distinct repo by construction, so the returned ref (a different
	// owner/repo than sourceRef) is already guaranteed to differ from it,
	// even before considering the commit sha.
	if err := s.client.CommitFiles(ctx, dstOwner, dstRepo, gitEntryBranch, files, gitEntryAuthorName, gitEntryAuthorEmail, msg); err != nil {
		return "", fmt.Errorf("git entry bundle store: fork: sync content into %s/%s: %w", dstOwner, dstRepo, err)
	}
	sha, err := s.client.LatestCommit(ctx, dstOwner, dstRepo, gitEntryBranch)
	if err != nil {
		return "", fmt.Errorf("git entry bundle store: fork: latest commit %s/%s: %w", dstOwner, dstRepo, err)
	}
	return EncodeGitCommitRef(dstOwner, dstRepo, sha), nil
}

/*
	===== legacy monorepo reader (pre-redesign) =====

	GitMonorepoBundleStore is NOT the active store anywhere post-redesign —
	it exists purely so Deps.LegacyBundles can still Read() a source_ref
	minted by the ORIGINAL single-repo GitBundleStore (content-addressed
	under bundles/<sha[:2]>/<sha>/... in the singleton instance-catalog repo,
	gitrepo.InstanceRepoName) during the lazy migration healSourceRef
	performs — see catalog.go's healSourceRef doc. Write/Fork are
	implemented (BundleStore requires them) but are never called on a
	LegacyBundles value in practice; they preserve the original monorepo
	scheme only for completeness/testability.
*/

// legacyGitSourceRefPrefix matches the retired monorepo ref shape
// ("git:<owner>/<repo>/<branch>/<hash>", 4 slash-delimited segments) as
// opposed to IsGitCommitRef's 2-segment "owner/repo@sha" shape — used by
// healSourceRef to decide whether a "git:"-prefixed legacy ref should be
// read via GitMonorepoBundleStore rather than treated as already-current.
const legacyGitSourceRefPrefix = "git:"

// IsLegacyMonorepoRef reports whether ref is the retired
// "git:<owner>/<repo>/<branch>/<hash>" monorepo shape (4 slash segments
// after the scheme) — distinct from IsGitCommitRef's "owner/repo@sha" shape
// (no "@", 4 "/"-segments instead of the commit-pinned form's 1).
func IsLegacyMonorepoRef(ref string) bool {
	if !strings.HasPrefix(ref, legacyGitSourceRefPrefix) {
		return false
	}
	if strings.Contains(ref, "@") {
		return false // that's IsGitCommitRef's shape, not this one.
	}
	rest := strings.TrimPrefix(ref, legacyGitSourceRefPrefix)
	return len(strings.Split(rest, "/")) == 4
}

// decodeLegacyMonorepoRef parses the retired 4-segment shape.
func decodeLegacyMonorepoRef(ref string) (owner, repo, branch, hash string, err error) {
	if !IsLegacyMonorepoRef(ref) {
		return "", "", "", "", fmt.Errorf("legacy monorepo ref %q: unrecognized shape", ref)
	}
	parts := strings.Split(strings.TrimPrefix(ref, legacyGitSourceRefPrefix), "/")
	owner, repo, branch, hash = parts[0], parts[1], parts[2], parts[3]
	if repo == "" || branch == "" || hash == "" {
		return "", "", "", "", fmt.Errorf("legacy monorepo ref %q: repo, branch and hash must be non-empty", ref)
	}
	return owner, repo, branch, hash, nil
}

func legacyGitBundleDir(hash string) string {
	if len(hash) < 2 {
		return "bundles/" + hash
	}
	return "bundles/" + hash[:2] + "/" + hash
}

// GitMonorepoBundleStore reads (only) the retired single-repo, content
// addressed bundle layout — see the section doc above.
type GitMonorepoBundleStore struct {
	client GitClient
}

var _ BundleStore = (*GitMonorepoBundleStore)(nil)

// NewGitMonorepoBundleStore returns a GitMonorepoBundleStore backed by
// client, for reading legacy refs only.
func NewGitMonorepoBundleStore(client GitClient) *GitMonorepoBundleStore {
	return &GitMonorepoBundleStore{client: client}
}

// Write is unused in practice (see the type doc) but implemented to satisfy
// BundleStore: it mirrors the original GitBundleStore.Write's
// content-addressing exactly, for test parity.
func (s *GitMonorepoBundleStore) Write(ctx context.Context, id BundleIdentity, _ string, files map[string][]byte) (string, error) {
	owner, repo := gitrepo.InstanceRepoOwner, gitrepo.InstanceRepoName
	hash := contentRef(files)
	ref := legacyGitSourceRefPrefix + fmt.Sprintf("%s/%s/%s/%s", owner, repo, gitrepo.InstanceRepoBranch, hash)
	dir := legacyGitBundleDir(hash)
	prefixed := make(map[string][]byte, len(files))
	for name, content := range files {
		prefixed[dir+"/"+name] = content
	}
	msg := fmt.Sprintf("chore: store legacy catalog bundle %s", hash[:min(12, len(hash))])
	if err := s.client.CommitFiles(ctx, owner, repo, gitrepo.InstanceRepoBranch, prefixed, gitEntryAuthorName, gitEntryAuthorEmail, msg); err != nil {
		return "", fmt.Errorf("git monorepo bundle store: commit %q: %w", ref, err)
	}
	_ = id // unused: legacy layout has no per-entry repo.
	return ref, nil
}

// Read reads a legacy monorepo ref's files back.
func (s *GitMonorepoBundleStore) Read(ctx context.Context, ref string) (map[string][]byte, error) {
	owner, repo, branch, hash, err := decodeLegacyMonorepoRef(ref)
	if err != nil {
		return nil, fmt.Errorf("git monorepo bundle store: %w", err)
	}
	dir := legacyGitBundleDir(hash)
	paths, err := s.client.ListTree(ctx, owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("git monorepo bundle store: list %q: %w", ref, err)
	}
	prefix := dir + "/"
	files := make(map[string][]byte)
	for _, p := range paths {
		rel, ok := strings.CutPrefix(p, prefix)
		if !ok || rel == "" {
			continue
		}
		content, err := s.client.GetFile(ctx, owner, repo, branch, p)
		if err != nil {
			return nil, fmt.Errorf("git monorepo bundle store: read %q: %w", p, err)
		}
		files[rel] = content
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("legacy monorepo ref %q not found", ref)
	}
	return files, nil
}

// Fork is unused in practice (see the type doc); implemented for interface
// completeness only, salted with a uuid exactly like the original
// GitBundleStore.Fork — the salt picks a distinct destination directory
// (so Fork(x) != x even for byte-identical content) but is never itself
// persisted into the stored bundle.
func (s *GitMonorepoBundleStore) Fork(ctx context.Context, _ BundleIdentity, sourceRef string) (string, error) {
	files, err := s.Read(ctx, sourceRef)
	if err != nil {
		return "", fmt.Errorf("git monorepo bundle store: fork: %w", err)
	}
	owner, repo := gitrepo.InstanceRepoOwner, gitrepo.InstanceRepoName
	salted := make(map[string][]byte, len(files)+1)
	for k, v := range files {
		salted[k] = v
	}
	salted["\x00fork-salt"] = []byte(sourceRef + "#" + uuid.NewString())
	hash := contentRef(salted)
	ref := legacyGitSourceRefPrefix + fmt.Sprintf("%s/%s/%s/%s", owner, repo, gitrepo.InstanceRepoBranch, hash)
	dir := legacyGitBundleDir(hash)
	prefixed := make(map[string][]byte, len(files))
	for name, content := range files {
		prefixed[dir+"/"+name] = content
	}
	msg := fmt.Sprintf("chore: fork legacy catalog bundle %s", hash[:min(12, len(hash))])
	if err := s.client.CommitFiles(ctx, owner, repo, gitrepo.InstanceRepoBranch, prefixed, gitEntryAuthorName, gitEntryAuthorEmail, msg); err != nil {
		return "", fmt.Errorf("git monorepo bundle store: fork commit %q: %w", ref, err)
	}
	return ref, nil
}
