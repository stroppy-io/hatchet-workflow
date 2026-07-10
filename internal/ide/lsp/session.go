package lsp

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"go.lsp.dev/uri"
)

// Session holds one LSP client connection's mutable state: its workspace
// root (set once, at `initialize`, from InitializeParams.RootURI — see
// server.go) and the in-memory overlay of currently-open, possibly-unsaved
// document buffers (textDocument/didOpen, didChange).
//
// Isolation: workspaceRoot is the ONLY filesystem root this Session will
// ever read from or resolve a URI against — see ResolvePath's doc comment.
// The stroppy-yaml-lsp binary itself runs as a subprocess inside a single
// scope's code-server container (internal/ide.Manager's per-scope
// volume-subpath mount, see .superpowers/sdd/spc-t4-report.md), so
// workspaceRoot is already confined to that one org's (or the instance's)
// worktree at the OS/container level; ResolvePath is this package's own,
// second, independent check that a request cannot walk the LSP process
// itself outside that root even if code-server or the client somehow
// forwarded a foreign path — belt and suspenders, not the only line of
// defense.
type Session struct {
	workspaceRoot string

	mu      sync.RWMutex
	overlay map[string][]byte // absolute, cleaned path -> unsaved buffer content
}

// NewSession constructs a Session rooted at workspaceRoot. workspaceRoot
// should be an absolute, already-cleaned path — server.go's `initialize`
// handler is responsible for resolving InitializeParams.RootURI/RootPath to
// one before calling this.
func NewSession(workspaceRoot string) *Session {
	return &Session{
		workspaceRoot: filepath.Clean(workspaceRoot),
		overlay:       map[string][]byte{},
	}
}

// ResolvePath decodes a file:// URI (or bare path) to an absolute,
// cleaned filesystem path and verifies it falls within the session's
// workspaceRoot, refusing (fail closed) otherwise. This is the isolation
// boundary the task's hard rules require: "an LSP session must only ever
// see the worktree of the scope it was authorized for ... make sure the LSP
// opens no side door" — see TestSession_ResolvePath_RejectsPathOutsideWorkspaceRoot
// and TestSession_ResolvePath_RejectsPathTraversalWithinURI.
func (s *Session) ResolvePath(rawURI string) (string, error) {
	path := uri.URI(rawURI).Filename()
	if path == "" {
		// Not a well-formed file:// URI; treat the raw string as a bare path
		// (some clients send plain paths for non-standard commands).
		path = rawURI
	}
	clean := filepath.Clean(path)

	rel, err := filepath.Rel(s.workspaceRoot, clean)
	if err != nil {
		return "", fmt.Errorf("resolve path %q against workspace root %q: %w", rawURI, s.workspaceRoot, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace root %q: refused", clean, s.workspaceRoot)
	}
	return clean, nil
}

// SetOverlay records path's current, possibly-unsaved buffer content
// (textDocument/didOpen or didChange with TextDocumentSyncKindFull, so
// Text is always the document's full content).
func (s *Session) SetOverlay(path string, content []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.overlay[filepath.Clean(path)] = content
}

// RemoveOverlay drops path's buffer content (textDocument/didClose) — later
// BundleFiles calls fall back to the on-disk content again.
func (s *Session) RemoveOverlay(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.overlay, filepath.Clean(path))
}

// BundleFiles finds the bundle root containing path (FindBundleRoot,
// bounded by the session's workspaceRoot) and snapshots it (SnapshotFiles),
// then overlays every currently-open document under that root with its
// live buffer content instead of what is on disk — so diagnostics/
// completion/preview reflect what the user is typing, not their last save.
func (s *Session) BundleFiles(path string) (bundleRoot string, files map[string][]byte, err error) {
	bundleRoot = FindBundleRoot(s.workspaceRoot, path)
	if bundleRoot == "" {
		return "", nil, fmt.Errorf("no cluster.yaml+workflow.yaml bundle found above %q (within workspace root %q)", path, s.workspaceRoot)
	}
	files, err = SnapshotFiles(bundleRoot)
	if err != nil {
		return "", nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for absPath, content := range s.overlay {
		rel, relErr := filepath.Rel(bundleRoot, absPath)
		if relErr != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue // overlay entry belongs to a different bundle; not relevant here
		}
		files[filepath.ToSlash(rel)] = content
	}
	return bundleRoot, files, nil
}
