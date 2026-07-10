package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/jsonrpc2/fake"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// notificationRecorder collects textDocument/publishDiagnostics
// notifications the server sends over the connection, so the end-to-end
// test below can assert on them without a real editor.
type notificationRecorder struct {
	mu    sync.Mutex
	diags []protocol.PublishDiagnosticsParams
}

func (r *notificationRecorder) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	if req.Method() == "textDocument/publishDiagnostics" {
		var p protocol.PublishDiagnosticsParams
		_ = json.Unmarshal(req.Params(), &p)
		r.mu.Lock()
		r.diags = append(r.diags, p)
		r.mu.Unlock()
	}
	return reply(ctx, nil, nil)
}

func (r *notificationRecorder) snapshot() []protocol.PublishDiagnosticsParams {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]protocol.PublishDiagnosticsParams, len(r.diags))
	copy(out, r.diags)
	return out
}

// TestServer_EndToEnd_InitializeOpenCompletePreview drives a real Server
// over an in-memory jsonrpc2 pipe (go.lsp.dev/jsonrpc2/fake), the same
// message framing cmd/stroppy-yaml-lsp/main.go uses over stdio — this is
// the "does the wiring actually work" proof the T5-T7 report calls for,
// distinct from the pure-function unit tests in diagnostics_test.go/
// completion_test.go/preview_test.go, which never touch jsonrpc2 at all.
//
// srv.conn is captured from the StreamServer callback (the connection the
// fake pipe server itself constructs for this session) rather than reusing
// the client's own Conn value — a jsonrpc2 connection is a single duplex
// object per side of the pipe; the server must Notify over ITS side, which
// is only available inside the StreamServer callback, not before Connect is
// called.
func TestServer_EndToEnd_InitializeOpenCompletePreview(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "recipe")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	clusterPath := filepath.Join(bundle, "cluster.yaml")
	workflowPath := filepath.Join(bundle, "workflow.yaml")
	if err := os.WriteFile(clusterPath, []byte("version: 1\n"+
		"provider:\n  use: docker\n"+
		"machines:\n  db:\n    count: 1\n    resources: { cpu: 2, ram: 2g, disk: { size: 10g, type: ssd } }\n"+
		"services:\n  postgres:\n    on: db\n    image: postgres:17\n    network: host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflowPath, []byte("inputs:\n  iterations: int\n"+
		"jobs:\n  postgres:\n    service: postgres\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := dsl.NewDslService()
	srv := NewServer(svc, nil)

	captured := make(chan struct{})
	var once sync.Once
	pipe := fake.NewPipeServer(ctx, jsonrpc2.ServerFunc(func(ctx context.Context, conn jsonrpc2.Conn) error {
		once.Do(func() {
			srv.conn = conn
			close(captured)
		})
		conn.Go(ctx, srv.Handle)
		<-conn.Done()
		return conn.Err()
	}), nil)

	client := pipe.Connect(ctx)
	rec := &notificationRecorder{}
	client.Go(ctx, rec.handle)

	select {
	case <-captured:
	case <-time.After(2 * time.Second):
		t.Fatal("server never captured its own connection")
	}

	// 1. initialize
	var initResult protocol.InitializeResult
	if _, err := client.Call(ctx, "initialize", protocol.InitializeParams{RootURI: uri.File(root)}, &initResult); err != nil {
		t.Fatalf("initialize call: %v", err)
	}
	if initResult.Capabilities.CompletionProvider == nil {
		t.Fatal("expected completion capability advertised")
	}
	if initResult.Capabilities.ExecuteCommandProvider == nil ||
		len(initResult.Capabilities.ExecuteCommandProvider.Commands) != 1 ||
		initResult.Capabilities.ExecuteCommandProvider.Commands[0] != previewCommand {
		t.Fatalf("expected the %q command advertised, got %+v", previewCommand, initResult.Capabilities.ExecuteCommandProvider)
	}

	// 2. didOpen the clean bundle -> expect an empty-diagnostics publish.
	if err := client.Notify(ctx, "textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  uri.File(clusterPath),
			Text: mustReadFile(t, clusterPath),
		},
	}); err != nil {
		t.Fatalf("didOpen notify: %v", err)
	}

	waitFor(t, func() bool {
		for _, p := range rec.snapshot() {
			if p.URI.Filename() == clusterPath {
				return true
			}
		}
		return false
	})
	for _, p := range rec.snapshot() {
		if p.URI.Filename() == clusterPath && len(p.Diagnostics) != 0 {
			t.Fatalf("expected zero diagnostics for a clean bundle, got %+v", p.Diagnostics)
		}
	}

	// 3. textDocument/completion -> top-level fields, including the
	// workflow input "iterations" hoisted by ComposeFormSchema.
	var completions protocol.CompletionList
	_, err := client.Call(ctx, "textDocument/completion", protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(clusterPath)},
		},
	}, &completions)
	if err != nil {
		t.Fatalf("completion call: %v", err)
	}
	var sawIterations bool
	for _, item := range completions.Items {
		if item.Label == "iterations" {
			sawIterations = true
		}
	}
	if !sawIterations {
		t.Fatalf("expected an %q completion item, got %+v", "iterations", completions.Items)
	}

	// 4. workspace/executeCommand(stroppy/previewBundle) -> a compiled plan.
	var preview PreviewResult
	_, err = client.Call(ctx, "workspace/executeCommand", protocol.ExecuteCommandParams{
		Command:   previewCommand,
		Arguments: []interface{}{string(uri.File(clusterPath))},
	}, &preview)
	if err != nil {
		t.Fatalf("executeCommand call: %v", err)
	}
	if preview.Plan == nil {
		t.Fatalf("expected a non-nil compiled plan, diagnostics: %+v", preview.Diagnostics)
	}
}

// TestServer_ExecuteCommand_RejectsPathOutsideWorkspaceRoot is the
// server-level companion to TestSession_ResolvePath_RejectsPathOutsideWorkspaceRoot
// — proving the JSON-RPC entry point itself refuses a request naming a
// document outside the session's workspace root, not just the Session type
// in isolation.
func TestServer_ExecuteCommand_RejectsPathOutsideWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	svc := dsl.NewDslService()
	srv := NewServer(svc, nil)
	srv.sess = NewSession(root)

	outside := filepath.Join(filepath.Dir(root), "other-scope", "cluster.yaml")

	var replied bool
	var replyErr error
	reply := func(_ context.Context, _ interface{}, err error) error {
		replied = true
		replyErr = err
		return nil
	}

	call, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), "workspace/executeCommand", protocol.ExecuteCommandParams{
		Command:   previewCommand,
		Arguments: []interface{}{string(uri.File(outside))},
	})
	if err != nil {
		t.Fatalf("build call: %v", err)
	}
	if err := srv.handleExecuteCommand(context.Background(), reply, call); err != nil {
		t.Fatalf("handleExecuteCommand: %v", err)
	}
	if !replied {
		t.Fatal("expected a reply")
	}
	if replyErr == nil {
		t.Fatal("expected an error rejecting the out-of-workspace path, got nil")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition never became true within 2s")
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test fixture only.
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
