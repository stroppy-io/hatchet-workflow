// Command stroppy-yaml-lsp is a stdio JSON-RPC 2.0 language server for the
// stroppy-yaml DSL (cluster.yaml/workflow.yaml bundles): diagnostics via
// DslService.Check, completion via DslService.ComposedSchema, and a
// stroppy/previewBundle command via DslService.Preview
// (internal/ide/lsp.Server — see that package's doc comment for why it does
// NOT implement go.lsp.dev/protocol's Server interface).
//
// It ships into a scope's code-server container image/worktree so an
// editor extension inside code-server can spawn it as a subprocess over
// stdio — see .superpowers/sdd/spc-t5-t7-report.md's "hosting" section for
// how it reaches code-server and why.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.lsp.dev/jsonrpc2"

	"github.com/stroppy-io/stroppy-cloud/internal/ide/lsp"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// stdrwc adapts os.Stdin/os.Stdout (two separate *os.File) to the single
// io.ReadWriteCloser jsonrpc2.NewStream expects. Close closes both — the
// same "process exit == stream close" semantics gopls and every other
// stdio LSP server use.
type stdrwc struct {
	in  io.ReadCloser
	out io.WriteCloser
}

func (s stdrwc) Read(p []byte) (int, error)  { return s.in.Read(p) }
func (s stdrwc) Write(p []byte) (int, error) { return s.out.Write(p) }

func (s stdrwc) Close() error {
	inErr := s.in.Close()
	outErr := s.out.Close()
	if inErr != nil {
		return inErr
	}
	return outErr
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stroppy-yaml-lsp:", err)
		os.Exit(1)
	}
}

func run() error {
	svc := dsl.NewDslService()
	stream := jsonrpc2.NewStream(stdrwc{in: os.Stdin, out: os.Stdout})
	conn := jsonrpc2.NewConn(stream)
	server := lsp.NewServer(svc, conn)
	return server.Run(context.Background())
}
