package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// previewCommand is the custom LSP command name (not a standard LSP method
// — spec §3 C3) an extension's webview invokes via workspace/executeCommand
// to render the compiled plan for the bundle containing a given document.
const previewCommand = "stroppy/previewBundle"

// diagnosticsDebounce mirrors dsl-editor.tsx's existing `{ delay: 400 }`
// linter convention (spec §3 C3's "every ~400ms of editing quiet") — a
// didChange notification schedules a re-check after this delay rather than
// recompiling on every keystroke; a later didChange for the same document
// cancels the pending timer and reschedules.
const diagnosticsDebounce = 400 * time.Millisecond

// Server is a hand-rolled JSON-RPC 2.0 dispatcher for the LSP methods
// stroppy-yaml needs (initialize, didOpen/didChange/didClose, completion,
// executeCommand). It deliberately does NOT implement go.lsp.dev/protocol's
// Server interface — see this package's doc comment in
// .superpowers/sdd/spc-t5-t7-report.md for why: that interface is wide
// (dozens of methods covering the full LSP surface, e.g. CodeAction, which
// a prior attempt at this package left unimplemented and never caught until
// `go build` failed). Handling exactly the methods this server supports,
// and returning jsonrpc2.ErrMethodNotFound for everything else, is both
// simpler and impossible to silently leave half-implemented.
//
// All actual domain logic (diagnostics conversion, completion derivation,
// preview) lives in diagnostics.go/completion.go/preview.go and is unit
// tested there without any jsonrpc2/protocol machinery involved; this file
// is wiring only.
type Server struct {
	svc  *dsl.DslService
	conn jsonrpc2.Conn
	sess *Session

	timersMu sync.Mutex
	timers   map[string]*time.Timer // absolute doc path -> pending debounced diagnostics timer
}

// NewServer constructs a Server bound to svc (the same DslService instance
// the connect RPC handler and every other in-process caller share — no
// separate compiler instantiation) and conn (a jsonrpc2 connection over
// stdio, typically — see cmd/stroppy-yaml-lsp/main.go). The workspace root
// is not known until the client's `initialize` request arrives, so sess
// starts nil and is created by handleInitialize.
func NewServer(svc *dsl.DslService, conn jsonrpc2.Conn) *Server {
	return &Server{svc: svc, conn: conn, timers: map[string]*time.Timer{}}
}

// Run drives the connection until it closes.
func (s *Server) Run(ctx context.Context) error {
	s.conn.Go(ctx, s.Handle)
	<-s.conn.Done()
	return s.conn.Err()
}

// Handle is a jsonrpc2.Handler: it dispatches every incoming request/
// notification to the one method below that understands it.
func (s *Server) Handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized", "$/setTrace", "exit", "shutdown":
		// No server-side action needed for any of these; still must reply
		// exactly once for a Call (shutdown), and it is harmless to reply
		// for a Notification too (jsonrpc2's Replier no-ops for those).
		return reply(ctx, nil, nil)
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, reply, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, reply, req)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, reply, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, reply, req)
	case "textDocument/completion":
		return s.handleCompletion(ctx, reply, req)
	case "workspace/executeCommand":
		return s.handleExecuteCommand(ctx, reply, req)
	default:
		return reply(ctx, nil, fmt.Errorf("%q: %w", req.Method(), jsonrpc2.ErrMethodNotFound))
	}
}

func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, fmt.Errorf("unmarshal initialize params: %w", err))
	}

	root := resolveWorkspaceRoot(params)
	if root == "" {
		return reply(ctx, nil, fmt.Errorf("initialize: no workspaceFolders/rootUri/rootPath given — the LSP session has no workspace to scope itself to"))
	}
	s.sess = NewSession(root)

	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncKindFull,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":", " "},
			},
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{previewCommand},
			},
		},
	}
	return reply(ctx, result, nil)
}

func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	path, err := s.resolveOrLog(string(params.TextDocument.URI))
	if err != nil {
		return reply(ctx, nil, nil)
	}
	s.sess.SetOverlay(path, []byte(params.TextDocument.Text))
	s.publishDiagnosticsNow(ctx, path)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	path, err := s.resolveOrLog(string(params.TextDocument.URI))
	if err != nil || len(params.ContentChanges) == 0 {
		return reply(ctx, nil, nil)
	}
	// TextDocumentSyncKindFull (advertised in handleInitialize) guarantees
	// the LAST content change carries the document's entire new text.
	full := params.ContentChanges[len(params.ContentChanges)-1].Text
	s.sess.SetOverlay(path, []byte(full))
	s.scheduleDiagnostics(ctx, path)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	if path, err := s.resolveOrLog(string(params.TextDocument.URI)); err == nil {
		s.publishDiagnosticsNow(ctx, path)
	}
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	if path, err := s.resolveOrLog(string(params.TextDocument.URI)); err == nil {
		s.sess.RemoveOverlay(path)
	}
	return reply(ctx, nil, nil)
}

func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	path, err := s.resolveOrLog(string(params.TextDocument.URI))
	if err != nil {
		return reply(ctx, protocol.CompletionList{}, nil)
	}
	_, files, err := s.sess.BundleFiles(path)
	if err != nil {
		return reply(ctx, protocol.CompletionList{}, nil)
	}
	items, err := CompletionItems(ctx, s.svc, files)
	if err != nil {
		return reply(ctx, protocol.CompletionList{}, nil)
	}
	return reply(ctx, protocol.CompletionList{Items: items}, nil)
}

func (s *Server) handleExecuteCommand(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.ExecuteCommandParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	if params.Command != previewCommand {
		return reply(ctx, nil, fmt.Errorf("%q: %w", params.Command, jsonrpc2.ErrMethodNotFound))
	}
	if len(params.Arguments) == 0 {
		return reply(ctx, nil, fmt.Errorf("%s: expected one argument (a document uri)", previewCommand))
	}
	rawURI, ok := params.Arguments[0].(string)
	if !ok {
		return reply(ctx, nil, fmt.Errorf("%s: argument must be a document uri string", previewCommand))
	}
	path, err := s.resolveOrLog(rawURI)
	if err != nil {
		return reply(ctx, nil, err)
	}
	_, files, err := s.sess.BundleFiles(path)
	if err != nil {
		return reply(ctx, nil, err)
	}
	result, err := Preview(ctx, s.svc, files)
	if err != nil {
		return reply(ctx, nil, err)
	}
	return reply(ctx, result, nil)
}

// resolveOrLog is Session.ResolvePath with the session-not-yet-initialized
// case folded in — every handler above needs both checks and none should
// ever proceed past either without a live session and an in-bounds path.
func (s *Server) resolveOrLog(rawURI string) (string, error) {
	if s.sess == nil {
		return "", fmt.Errorf("no active session (client sent a request before initialize)")
	}
	return s.sess.ResolvePath(rawURI)
}

// scheduleDiagnostics debounces publishDiagnosticsNow by diagnosticsDebounce
// per document path, matching dsl-editor.tsx's existing 400ms linter
// convention (see the package-level doc comment). Safe for concurrent
// didChange notifications on different documents; a later didChange on the
// SAME path cancels its own still-pending timer before scheduling a new one.
func (s *Server) scheduleDiagnostics(ctx context.Context, path string) {
	s.timersMu.Lock()
	defer s.timersMu.Unlock()
	if t, ok := s.timers[path]; ok {
		t.Stop()
	}
	s.timers[path] = time.AfterFunc(diagnosticsDebounce, func() {
		s.publishDiagnosticsNow(ctx, path)
	})
}

// publishDiagnosticsNow computes and sends textDocument/publishDiagnostics
// immediately (no debounce) — used directly by didOpen/didSave, and as the
// debounced timer's callback for didChange. It publishes for EVERY file in
// the bundle that Check reported a diagnostic for (a change in one
// component file can surface a diagnostic on another, e.g. cluster.yaml
// referencing an undefined component), plus an explicit empty array for the
// document that was just edited when Check reported nothing for it — so a
// bundle that just became clean has its squiggles cleared, not left stale.
func (s *Server) publishDiagnosticsNow(ctx context.Context, path string) {
	bundleRoot, files, err := s.sess.BundleFiles(path)
	if err != nil {
		// No bundle found above this document (e.g. a loose .md file) — there
		// is nothing to check; publish nothing rather than an empty-but-wrong
		// diagnostics set for an unrelated document.
		return
	}
	byPath, err := CheckDiagnostics(ctx, s.svc, files)
	if err != nil {
		return
	}

	editedRel, relErr := filepath.Rel(bundleRoot, path)
	editedRel = filepath.ToSlash(editedRel)
	published := map[string]bool{}
	for relPath, diags := range byPath {
		s.notify(ctx, uri.File(filepath.Join(bundleRoot, filepath.FromSlash(relPath))), diags)
		published[relPath] = true
	}
	if relErr == nil && !published[editedRel] {
		s.notify(ctx, uri.File(path), []protocol.Diagnostic{})
	}
}

// notify sends a single textDocument/publishDiagnostics notification and
// logs (rather than silently discards — errcheck's check-blank setting in
// this repo's .golangci.yaml requires every error be handled, not just
// blank-assigned) any transport failure. A broken client pipe is not this
// server's problem to recover from — Run's own conn.Done()/conn.Err() will
// already be unwinding the process by the time a Notify starts failing — so
// logging and continuing is the correct response, not a panic or an
// os.Exit from deep inside a debounce timer callback.
func (s *Server) notify(ctx context.Context, docURI protocol.DocumentURI, diags []protocol.Diagnostic) {
	if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         docURI,
		Diagnostics: diags,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "stroppy-yaml-lsp: publishDiagnostics(%s): %v\n", docURI, err)
	}
}

// resolveWorkspaceRoot prefers the modern, non-deprecated
// InitializeParams.WorkspaceFolders (its first entry — this server, like
// the rest of the stroppy-yaml toolchain, only ever scopes one bundle tree
// per session) and falls back to the deprecated RootURI/RootPath fields for
// any client that still only sends those (many do; LSP 3.x kept them for
// compatibility even after WorkspaceFolders superseded them).
func resolveWorkspaceRoot(params protocol.InitializeParams) string {
	if len(params.WorkspaceFolders) > 0 {
		if root := uri.URI(params.WorkspaceFolders[0].URI).Filename(); root != "" {
			return root
		}
	}
	if root := params.RootURI.Filename(); root != "" { //nolint:staticcheck // deprecated-field fallback for older clients, by design.
		return root
	}
	return params.RootPath //nolint:staticcheck // deprecated-field fallback for older clients, by design.
}
