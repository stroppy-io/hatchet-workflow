package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// postgresHADir is the same golden fixture internal/services/dsl/service_test.go
// uses (loadBundle(t, postgresHADir)) — reused here rather than fabricated,
// per the plan's own instruction not to hand-write a new malformed bundle
// when a real fixture already exercises the exact RPC path being tested.
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
		content, err := os.ReadFile(p) //nolint:gosec // test fixture only.
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

func TestCheckDiagnostics_CleanBundleHasNoDiagnostics(t *testing.T) {
	svc := dsl.NewDslService()
	byPath, err := CheckDiagnostics(context.Background(), svc, loadBundle(t, postgresHADir))
	if err != nil {
		t.Fatalf("check diagnostics: %v", err)
	}
	if len(byPath) != 0 {
		t.Fatalf("expected zero diagnostics for the clean postgres-ha golden bundle, got %+v", byPath)
	}
}

// TestCheckDiagnostics_GroupsByPathAndConvertsSeverity reuses the exact
// scenario internal/services/dsl/service_test.go's
// TestCheckMissingProviderManifestIsDiagnosticNotError proves produces a
// real ERROR diagnostic on "yandex" (module) — deleting
// providers/yandex/manifest.yaml from the golden bundle. This asserts the
// LSP conversion layer (path grouping + severity mapping), not the
// compiler's own diagnostic content, which that other test already covers.
func TestCheckDiagnostics_GroupsByPathAndConvertsSeverity(t *testing.T) {
	svc := dsl.NewDslService()
	files := loadBundle(t, postgresHADir)
	delete(files, "providers/yandex/manifest.yaml")

	byPath, err := CheckDiagnostics(context.Background(), svc, files)
	if err != nil {
		t.Fatalf("check diagnostics: %v", err)
	}
	if len(byPath) == 0 {
		t.Fatalf("expected at least one path with diagnostics for a missing provider manifest")
	}
	var found bool
	for path, diags := range byPath {
		for _, d := range diags {
			if d.Severity != protocol.DiagnosticSeverityError {
				continue
			}
			found = true
			_ = path
		}
	}
	if !found {
		t.Fatalf("expected an ERROR-severity LSP diagnostic among %+v", byPath)
	}
}

func TestClampLineCol_ZeroStaysZero_OneBasedShiftsDown(t *testing.T) {
	if got := clampLSPLine(0); got != 0 {
		t.Fatalf("clampLSPLine(0) = %d, want 0 (unknown position stays at document start)", got)
	}
	if got := clampLSPLine(1); got != 0 {
		t.Fatalf("clampLSPLine(1) = %d, want 0 (1-based line 1 -> 0-based line 0)", got)
	}
	if got := clampLSPLine(5); got != 4 {
		t.Fatalf("clampLSPLine(5) = %d, want 4", got)
	}
}

func TestToLSPSeverity_MapsAllThreeWireSeverities(t *testing.T) {
	cases := []struct {
		in   dslpb.Severity
		want protocol.DiagnosticSeverity
	}{
		{dslpb.Severity_SEVERITY_ERROR, protocol.DiagnosticSeverityError},
		{dslpb.Severity_SEVERITY_WARNING, protocol.DiagnosticSeverityWarning},
		{dslpb.Severity_SEVERITY_UNSPECIFIED, protocol.DiagnosticSeverityInformation},
	}
	for _, c := range cases {
		if got := toLSPSeverity(c.in); got != c.want {
			t.Fatalf("toLSPSeverity(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
