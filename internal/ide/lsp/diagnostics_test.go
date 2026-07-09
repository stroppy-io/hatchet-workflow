package lsp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// postgresHADir is the same golden recipe bundle fixture
// internal/services/dsl/service_test.go uses (examples/dsl/postgres-ha),
// reused here rather than a fabricated bundle so this package's tests
// cannot silently drift from what the real compiler asserts. Path is
// relative to this package's own directory (internal/ide/lsp), one level
// deeper than internal/services/dsl.
const postgresHADir = "../../../examples/dsl/postgres-ha"

func loadBundle(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(p) //nolint:gosec // test-only, reading our own fixture tree.
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		t.Fatalf("loadBundle(%q): %v", dir, err)
	}
	return files
}

func TestCheckDiagnostics_CleanBundleHasNone(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := loadBundle(t, postgresHADir)

	byPath, err := CheckDiagnostics(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("check diagnostics: %v", err)
	}
	if len(byPath) != 0 {
		t.Fatalf("expected no diagnostics for a clean bundle, got %+v", byPath)
	}
}

// TestCheckDiagnostics_GroupsByPathAndConvertsSeverity reuses the exact
// same etcd-quorum-violation fixture mutation
// internal/services/dsl/service_test.go's TestCheckEtcdQuorumViolationDiagnostic
// asserts against (patch cluster.yaml's "count: 3" -> "count: 2"), so this
// test cannot assert a diagnostic shape the real DslService.Check contract
// doesn't actually produce.
func TestCheckDiagnostics_GroupsByPathAndConvertsSeverity(t *testing.T) {
	svc := dsl.NewDslService()
	bundle := loadBundle(t, postgresHADir)

	cluster, ok := bundle["cluster.yaml"]
	if !ok {
		t.Fatal("bundle missing cluster.yaml")
	}
	patched := bytes.Replace(cluster, []byte("count: 3"), []byte("count: 2"), 1)
	if bytes.Equal(patched, cluster) {
		t.Fatal("patch did not change cluster.yaml — fixture drifted from the expected \"count: 3\" text")
	}
	bundle["cluster.yaml"] = patched

	byPath, err := CheckDiagnostics(context.Background(), svc, bundle)
	if err != nil {
		t.Fatalf("check diagnostics: %v", err)
	}

	var found protocol.Diagnostic
	var ok2 bool
	for _, ds := range byPath {
		for _, d := range ds {
			if d.Source == "etcd" && d.Severity == protocol.DiagnosticSeverityError {
				found, ok2 = d, true
			}
		}
	}
	if !ok2 {
		t.Fatalf("expected an etcd-sourced error diagnostic somewhere in %+v", byPath)
	}
	if found.Message == "" {
		t.Fatal("expected a non-empty message")
	}
	// KNOWN GAP (see clampToZeroBased's doc comment): the contract checker
	// that produces this diagnostic sets no Pos, so it must land at the
	// file-start Position{0,0} — asserted explicitly here so a future change
	// to contract/check.go that starts setting a real Pos is caught (this
	// assertion would then need updating, which is the point).
	if found.Range.Start.Line != 0 || found.Range.Start.Character != 0 {
		t.Fatalf("expected Position{0,0} for a contract-check diagnostic with no source Pos, got %+v", found.Range.Start)
	}
}
