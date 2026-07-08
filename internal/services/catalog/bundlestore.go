package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// BundleStore is the SP-C seam: catalog owns no git of its own.
// catalog_entries.source_ref is the opaque pointer BundleStore returns from
// Write/Fork and consumes in Read — this package never interprets it beyond
// passing it through. SP-C will swap in a gitea-backed implementation
// without this package (or its callers) changing.
type BundleStore interface {
	// Write stores files under a new ref and returns it. ref is currently
	// unused by both implementations (reserved for a future
	// update-in-place/branch-aware backend); callers pass "" today.
	Write(ctx context.Context, ref string, files map[string][]byte) (newRef string, err error)
	// Read returns the files stored at ref. The returned map is owned by the
	// caller — mutating it must never affect the store's contents.
	Read(ctx context.Context, ref string) (map[string][]byte, error)
	// Fork copies sourceRef's contents to a brand-new ref, independent of the
	// source from that point on. The new ref MUST differ from sourceRef even
	// when contents are byte-identical.
	Fork(ctx context.Context, sourceRef string) (forkedRef string, err error)
}

// MemoryBundleStore is an in-process, content-addressed BundleStore used by
// tests (and any caller that does not need durability). It deep-copies on
// both Write and Read so a caller mutating a map it passed in, or a map it
// got back, can never corrupt the store's internal state.
type MemoryBundleStore struct {
	mu    sync.RWMutex
	store map[string]map[string][]byte
}

// NewMemoryBundleStore returns an empty, ready-to-use MemoryBundleStore.
func NewMemoryBundleStore() *MemoryBundleStore {
	return &MemoryBundleStore{store: make(map[string]map[string][]byte)}
}

var _ BundleStore = (*MemoryBundleStore)(nil)

// Write stores a deep copy of files under files' content ref and returns it.
func (m *MemoryBundleStore) Write(_ context.Context, _ string, files map[string][]byte) (string, error) {
	ref := contentRef(files)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[ref] = deepCopyFiles(files)
	return ref, nil
}

// Read returns a deep copy of the files stored at ref.
func (m *MemoryBundleStore) Read(_ context.Context, ref string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	files, ok := m.store[ref]
	if !ok {
		return nil, fmt.Errorf("bundle ref %q not found", ref)
	}
	return deepCopyFiles(files), nil
}

// Fork copies sourceRef's contents into a new ref. The new ref is salted so
// it always differs from sourceRef, even when the forked bundle is
// byte-identical to the source (fork-on-edit needs a distinct ref
// immediately, before any edit happens).
func (m *MemoryBundleStore) Fork(ctx context.Context, sourceRef string) (string, error) {
	files, err := m.Read(ctx, sourceRef)
	if err != nil {
		return "", fmt.Errorf("fork: %w", err)
	}

	m.mu.Lock()
	saltedRef := forkRef(sourceRef, files, len(m.store))
	m.store[saltedRef] = deepCopyFiles(files)
	m.mu.Unlock()
	return saltedRef, nil
}

// FSBundleStore is a local-filesystem-backed, content-addressed BundleStore
// for dev/staging use: each ref is a subdirectory of root named by its
// content hash, containing one file per bundle entry (nested paths
// preserved).
type FSBundleStore struct {
	root string
}

// NewFSBundleStore returns an FSBundleStore rooted at root, creating root if
// it does not already exist.
func NewFSBundleStore(root string) (*FSBundleStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("fs bundle store: %w", err)
	}
	return &FSBundleStore{root: root}, nil
}

var _ BundleStore = (*FSBundleStore)(nil)

// Write stores files under root/<contentRef>/... and returns the ref.
func (s *FSBundleStore) Write(_ context.Context, _ string, files map[string][]byte) (string, error) {
	ref := contentRef(files)
	if err := s.writeRef(ref, files); err != nil {
		return "", err
	}
	return ref, nil
}

// Read walks root/<ref> back into a map[string][]byte.
func (s *FSBundleStore) Read(_ context.Context, ref string) (map[string][]byte, error) {
	dir, err := s.refDir(ref)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("bundle ref %q not found", ref)
	}

	files := make(map[string][]byte)
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path) //nolint:gosec // path is derived from Walk over a ref dir this store itself created.
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read bundle ref %q: %w", ref, err)
	}
	return files, nil
}

// Fork reads sourceRef and re-Writes it under a salted ref, matching
// MemoryBundleStore.Fork's salt trick so identical content still forks to a
// new ref.
func (s *FSBundleStore) Fork(ctx context.Context, sourceRef string) (string, error) {
	files, err := s.Read(ctx, sourceRef)
	if err != nil {
		return "", fmt.Errorf("fork: %w", err)
	}

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return "", fmt.Errorf("fork: %w", err)
	}
	saltedRef := forkRef(sourceRef, files, len(entries))
	if err := s.writeRef(saltedRef, files); err != nil {
		return "", fmt.Errorf("fork: %w", err)
	}
	return saltedRef, nil
}

// writeRef materializes files under root/<ref>/..., creating parent
// directories as needed. files entries are caller/ref-derived; refDir
// already rejects any ref that escapes root, and every per-file relative
// path is required to be filepath.IsLocal before being joined onto the ref
// directory, so a malicious or malformed key cannot write outside the ref
// directory (mirrors internal/services/dsl's materializeProviderModule
// guard).
func (s *FSBundleStore) writeRef(ref string, files map[string][]byte) error {
	dir, err := s.refDir(ref)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("write bundle ref %q: %w", ref, err)
	}
	for _, name := range sortedKeys(files) {
		relOS := filepath.FromSlash(name)
		if !filepath.IsLocal(relOS) {
			return fmt.Errorf("write bundle ref %q: file %q escapes the bundle directory (path traversal)", ref, name)
		}
		dest := filepath.Join(dir, relOS)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("write bundle ref %q: %w", ref, err)
		}
		if err := os.WriteFile(dest, files[name], 0o644); err != nil { //nolint:gosec // bundle content is not secret; 0o644 matches sibling FS writes in this codebase.
			return fmt.Errorf("write bundle ref %q: %w", ref, err)
		}
	}
	return nil
}

// refDir resolves ref to a directory under root, rejecting any ref that
// would escape root (path traversal via ".." or an absolute path).
func (s *FSBundleStore) refDir(ref string) (string, error) {
	relOS := filepath.FromSlash(ref)
	if ref == "" || !filepath.IsLocal(relOS) {
		return "", fmt.Errorf("bundle ref %q is invalid", ref)
	}
	return filepath.Join(s.root, relOS), nil
}

// contentRef derives a stable, content-addressed ref from files: the hex
// SHA-256 of each (name, content) pair in sorted-name order.
func contentRef(files map[string][]byte) string {
	h := sha256.New()
	for _, name := range sortedKeys(files) {
		h.Write([]byte(name))
		h.Write(files[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// forkRef derives a fork's ref by salting files with sourceRef and a
// disambiguator (typically the store's current size) before hashing, then
// discarding the salt — so Fork(x) always differs from x, even when x's
// contents are byte-identical to the fork, without that salt leaking into
// the stored bundle.
func forkRef(sourceRef string, files map[string][]byte, disambiguator int) string {
	salted := make(map[string][]byte, len(files)+1)
	for k, v := range files {
		salted[k] = v
	}
	salted["\x00fork-salt"] = []byte(fmt.Sprintf("%s#%d", sourceRef, disambiguator))
	return contentRef(salted)
}

// deepCopyFiles returns a copy of files where every byte slice is its own
// independent allocation, so mutating the copy (or the original) never
// affects the other.
func deepCopyFiles(files map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for k, v := range files {
		cp := make([]byte, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// sortedKeys returns files' keys in sorted order, for deterministic hashing
// and deterministic filesystem writes.
func sortedKeys(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
