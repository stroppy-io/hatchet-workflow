package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GitClient is the narrow subset of *internal/gitrepo.Client GitBundleStore
// needs (CommitFiles/GetFile/ListTree — the same three primitives T1/T2
// built and documented as "the primitives a future GitBundleStore
// implementing this interface would call", see bundlestore.go's BundleStore
// doc). Declaring it here (rather than importing *gitrepo.Client directly)
// keeps this file trivially testable with a fake, exactly like Checker/
// CatalogEntryRepo above are narrow interfaces rather than concrete types.
// *gitrepo.Client already satisfies this interface structurally (method
// signatures match byte-for-byte) — no change to internal/gitrepo was
// needed or made.
type GitClient interface {
	CommitFiles(ctx context.Context, owner, repo, branch string, files map[string][]byte, author, email, message string) error
	GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error)
	ListTree(ctx context.Context, owner, repo, ref string) ([]string, error)
}

// GitBundleStore is a BundleStore backed by a single Gitea repo
// (owner/repo/branch), content-addressed the same way FSBundleStore is:
// every distinct (sorted-name, content) set of files hashes to one ref, and
// identical bundles collapse onto the same git path instead of growing the
// repo's history on every write.
//
// # source_ref encoding
//
// A ref returned by Write/Fork has the form:
//
//	git:<owner>/<repo>/<branch>/<sha256-hex>
//
// It is fully self-describing (owner, repo, branch, and the content hash are
// all recoverable from the string alone via DecodeGitSourceRef) — but Read
// and Fork DELIBERATELY refuse to honor a ref naming any (owner, repo,
// branch) other than this store's own: decoding is used only to validate
// the ref and extract the content hash, never to redirect the underlying
// GitClient call to a different repo. This is the multi-tenancy enforcement
// point (see the package-level test TestGitBundleStore_CrossRepoIsolation):
// a GitBundleStore constructed for org-a's repo cannot be tricked into
// reading org-b's repo by handing it a ref that merely names org-b — every
// call is pinned to the (owner, repo, branch) passed to NewGitBundleStore,
// full stop. (An earlier version of this file let Read follow whatever
// owner/repo a ref named, on the theory that "the ref is self-describing" —
// that is wrong for a multi-tenant store: the ref is caller-suppliable data
// once it round-trips through GetOrgProviderFiles/GetOrgWorkflowFiles, and
// trusting caller-suppliable data to pick which repo a shared Gitea token
// reads is exactly the kind of cross-tenant hole this task's hard rules
// call out. Pinning to the constructed scope closes it.) Write is likewise
// pinned to this store's own (owner, repo, branch); the BundleStore
// interface's Write(ctx, ref, files) receives no tenant/scope hint anyway
// (see bundlestore.go's doc: "ref is currently unused by both
// implementations (reserved for a future ... backend); callers pass \"\"
// today").
//
// # Why this is a single-repo store, not "one repo per org"
//
// The spec's product decision (an org repo per tenant, empty until forked,
// live-linked to the instance repo otherwise) is a property of
// CatalogEntry rows, not of BundleStore: a LINKED row simply carries no
// SourceRef of its own and falls back to its source instance row's ref
// (service.go's resolveSourceRef) — no bytes are copied, so nothing needs
// to exist in an "org repo" yet. A fork (newVersion) always calls
// Write(ctx, "", files) and gets back a brand-new ref pointing at
// content the original row's ref never referenced — that is the
// "diverges independently" property, satisfied by content-addressing
// alone, regardless of how many physical git repos back the store.
//
// Wiring ONE GitBundleStore per tenant (each pointed at that tenant's own
// Gitea repo) *would* additionally get the literal "org's own git repo
// holds only that org's forked bundles" property — but catalog.Deps.Bundles
// today is a single field shared by every CreateOrgEntry/UpdateOrgEntry
// call across every tenant (internal/app/run.go wires exactly one
// catalogBundles for the whole server), and neither Write nor the RPCs that
// call it thread a tenant ID down to BundleStore. Giving BundleStore that
// tenant-scoping would mean widening the interface T2 shipped (and every
// call site) — out of this task's authorized surface ("slot beneath
// service.go, do NOT duplicate or bypass the RBAC those RPCs enforce").
// GitBundleStore is therefore wired as ONE store over the instance repo
// (see internal/app/run.go) exactly where FSBundleStore was: it is a
// drop-in, content-addressed, git-backed replacement for FS storage, not a
// per-tenant repo router. The literal "org has its own real git
// repository" requirement is met one layer up, by internal/ide's per-org
// worktrees (see internal/ide/worktree.go) — those are real Gitea repos a
// human edits directly via code-server, entirely separate from this
// content-addressed bundle cache the catalog RPCs read/write through.
type GitBundleStore struct {
	client                  GitClient
	owner, repo, branch     string
	authorName, authorEmail string
}

var _ BundleStore = (*GitBundleStore)(nil)

// NewGitBundleStore returns a GitBundleStore that writes into owner/repo on
// branch. The repo must already exist (gitrepo.Client.EnsureRepo /
// gitrepo.Bootstrap is the caller's responsibility, mirroring how
// FSBundleStore's caller MkdirAlls its root before use).
func NewGitBundleStore(client GitClient, owner, repo, branch string) *GitBundleStore {
	return &GitBundleStore{
		client:      client,
		owner:       owner,
		repo:        repo,
		branch:      branch,
		authorName:  "stroppy-bot",
		authorEmail: "bot@stroppy.local",
	}
}

// gitSourceRefPrefix distinguishes a git-backed ref from a plain
// content-hash ref (FSBundleStore/MemoryBundleStore's format) so a caller
// holding an opaque ref string can never confuse the two.
const gitSourceRefPrefix = "git:"

// EncodeGitSourceRef builds the source_ref string for (owner, repo, branch,
// hash). Exported so callers that need to construct/inspect a git-backed ref
// outside this package (e.g. internal/ide, tests) do not have to
// hand-format the scheme.
func EncodeGitSourceRef(owner, repo, branch, hash string) string {
	return fmt.Sprintf("%s%s/%s/%s/%s", gitSourceRefPrefix, owner, repo, branch, hash)
}

// DecodeGitSourceRef parses a ref produced by EncodeGitSourceRef. owner may
// legitimately be empty (the instance repo's owner, gitrepo.
// InstanceRepoOwner) — repo, branch and hash must not be.
func DecodeGitSourceRef(ref string) (owner, repo, branch, hash string, err error) {
	if !strings.HasPrefix(ref, gitSourceRefPrefix) {
		return "", "", "", "", fmt.Errorf("git bundle ref %q: missing %q scheme", ref, gitSourceRefPrefix)
	}
	rest := strings.TrimPrefix(ref, gitSourceRefPrefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("git bundle ref %q: expected owner/repo/branch/hash", ref)
	}
	owner, repo, branch, hash = parts[0], parts[1], parts[2], parts[3]
	if repo == "" || branch == "" || hash == "" {
		return "", "", "", "", fmt.Errorf("git bundle ref %q: repo, branch and hash must be non-empty", ref)
	}
	return owner, repo, branch, hash, nil
}

// IsGitSourceRef reports whether ref was minted by a GitBundleStore, so a
// caller juggling multiple BundleStore backends (e.g. during a migration)
// can branch on ref shape without a type assertion on the store itself.
func IsGitSourceRef(ref string) bool {
	return strings.HasPrefix(ref, gitSourceRefPrefix)
}

// gitBundleDir is the repo-relative directory a bundle's files live under,
// sharded by the hash's first two hex characters (same fan-out convention
// FSBundleStore's flat root would benefit from at scale; done here upfront
// since a git tree listing is O(entries in the ref) whereas a directory scan
// is not free the way a local filesystem's is).
func gitBundleDir(hash string) string {
	if len(hash) < 2 {
		return "bundles/" + hash
	}
	return "bundles/" + hash[:2] + "/" + hash
}

// Write stores files content-addressed under this store's own (owner, repo,
// branch) — see the type doc for why Write cannot honor an arbitrary
// destination the way Read/Fork can. Re-writing byte-identical content is a
// no-op against Gitea (checked via ListTree before committing) so repeated
// saves of unchanged bundles do not spam the repo's commit history.
func (s *GitBundleStore) Write(ctx context.Context, _ string, files map[string][]byte) (string, error) {
	hash := contentRef(files)
	ref := EncodeGitSourceRef(s.owner, s.repo, s.branch, hash)
	exists, err := s.hasBundle(ctx, s.owner, s.repo, s.branch, hash)
	if err != nil {
		return "", fmt.Errorf("git bundle store: check existing %q: %w", ref, err)
	}
	if exists {
		return ref, nil
	}
	dir := gitBundleDir(hash)
	prefixed := make(map[string][]byte, len(files))
	for name, content := range files {
		prefixed[dir+"/"+name] = content
	}
	msg := fmt.Sprintf("chore: store catalog bundle %s", hash[:min(12, len(hash))])
	if err := s.client.CommitFiles(ctx, s.owner, s.repo, s.branch, prefixed, s.authorName, s.authorEmail, msg); err != nil {
		return "", fmt.Errorf("git bundle store: commit %q: %w", ref, err)
	}
	return ref, nil
}

// Read decodes ref, verifies it names THIS store's own (owner, repo,
// branch) — see the type doc's isolation note — and reads every file under
// its bundle directory back into a map keyed by the original relative path.
func (s *GitBundleStore) Read(ctx context.Context, ref string) (map[string][]byte, error) {
	owner, repo, branch, hash, err := DecodeGitSourceRef(ref)
	if err != nil {
		return nil, fmt.Errorf("git bundle store: %w", err)
	}
	if owner != s.owner || repo != s.repo || branch != s.branch {
		return nil, fmt.Errorf("git bundle ref %q: names a different repo than this store owns (%s/%s/%s)", ref, s.owner, s.repo, s.branch)
	}
	dir := gitBundleDir(hash)
	paths, err := s.client.ListTree(ctx, owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("git bundle store: list %q: %w", ref, err)
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
			return nil, fmt.Errorf("git bundle store: read %q: %w", p, err)
		}
		files[rel] = content
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("git bundle ref %q not found", ref)
	}
	return files, nil
}

// Fork reads sourceRef's files and re-Writes them under a freshly salted
// hash into THIS store's own (owner, repo, branch) — so Fork(x) always
// yields a ref distinct from x, exactly like MemoryBundleStore/
// FSBundleStore.Fork guarantee, but salted with a uuid rather than a
// store-size counter (a git-backed store keeps no in-process index to size).
func (s *GitBundleStore) Fork(ctx context.Context, sourceRef string) (string, error) {
	files, err := s.Read(ctx, sourceRef)
	if err != nil {
		return "", fmt.Errorf("git bundle store: fork: %w", err)
	}
	salted := make(map[string][]byte, len(files)+1)
	for k, v := range files {
		salted[k] = v
	}
	salted["\x00fork-salt"] = []byte(sourceRef + "#" + uuid.NewString())
	// Compute the salted hash to decide the destination directory, but store
	// the UNSALTED files there (the salt must never leak into the persisted
	// bundle content) — same two-step Write MemoryBundleStore/FSBundleStore.
	// Fork perform via forkRef, done inline here since GitBundleStore's Write
	// re-derives its own hash from files and must not see the salt key.
	hash := contentRef(salted)
	ref := EncodeGitSourceRef(s.owner, s.repo, s.branch, hash)
	exists, err := s.hasBundle(ctx, s.owner, s.repo, s.branch, hash)
	if err != nil {
		return "", fmt.Errorf("git bundle store: fork: check existing %q: %w", ref, err)
	}
	if exists {
		return ref, nil
	}
	dir := gitBundleDir(hash)
	prefixed := make(map[string][]byte, len(files))
	for name, content := range files {
		prefixed[dir+"/"+name] = content
	}
	msg := fmt.Sprintf("chore: fork catalog bundle %s", hash[:min(12, len(hash))])
	if err := s.client.CommitFiles(ctx, s.owner, s.repo, s.branch, prefixed, s.authorName, s.authorEmail, msg); err != nil {
		return "", fmt.Errorf("git bundle store: fork commit %q: %w", ref, err)
	}
	return ref, nil
}

// hasBundle reports whether a bundle directory for hash already has at
// least one file committed on branch.
func (s *GitBundleStore) hasBundle(ctx context.Context, owner, repo, branch, hash string) (bool, error) {
	paths, err := s.client.ListTree(ctx, owner, repo, branch)
	if err != nil {
		return false, err
	}
	prefix := gitBundleDir(hash) + "/"
	for _, p := range paths {
		if strings.HasPrefix(p, prefix) {
			return true, nil
		}
	}
	return false, nil
}
