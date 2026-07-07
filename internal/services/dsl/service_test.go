package dsl

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// postgresHADir is the golden recipe bundle from examples/dsl/postgres-ha
// (the same fixture internal/dsl/golden_test.go compiles), reused here to
// exercise the connect handler end to end rather than the bare dsl.Compile
// facade.
const postgresHADir = "../../../examples/dsl/postgres-ha"

// loadBundle walks dir and returns every regular file's contents keyed by
// its slash path relative to dir, matching dslpb.CheckRequest/
// ComposedSchemaRequest's "files" map shape.
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
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(p) //nolint:gosec // test-only, reading our own fixture tree.
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		t.Fatalf("loadBundle(%q): %v", dir, err)
	}
	return files
}

func TestCheckPostgresHAZeroDiagnostics(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check returned an RPC error: %v", err)
	}
	if len(resp.GetDiagnostics()) != 0 {
		t.Fatalf("expected 0 diagnostics for a clean bundle, got %+v", resp.GetDiagnostics())
	}
}

func TestCheckEtcdQuorumViolationDiagnostic(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	// The etcd component requires its "nodes" machine group to have an odd
	// count >= 3 (see components/etcd/component.yaml's `requires:`). Patching
	// the db group's count from 3 to 2 must trip that contract check.
	cluster, ok := files["cluster.yaml"]
	if !ok {
		t.Fatal("bundle missing cluster.yaml")
	}
	patched := bytes.Replace(cluster, []byte("count: 3"), []byte("count: 2"), 1)
	if bytes.Equal(patched, cluster) {
		t.Fatal("patch did not change cluster.yaml — fixture drifted from the expected \"count: 3\" text")
	}
	files["cluster.yaml"] = patched

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check returned an RPC error: %v", err)
	}

	var found *dslpb.Diagnostic
	for _, d := range resp.GetDiagnostics() {
		if d.GetModule() == "etcd" && d.GetSeverity() == dslpb.Severity_SEVERITY_ERROR {
			found = d
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an ERROR diagnostic with module \"etcd\", got %+v", resp.GetDiagnostics())
	}
}

func TestPreviewPostgresHAReturnsResolvedPlan(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	resp, err := svc.Preview(context.Background(), &dslpb.PreviewRequest{Files: files})
	if err != nil {
		t.Fatalf("Preview returned an RPC error: %v", err)
	}
	if len(resp.GetDiagnostics()) != 0 {
		t.Fatalf("expected 0 diagnostics for a clean bundle, got %+v", resp.GetDiagnostics())
	}

	plan := resp.GetPlan()
	if plan == nil {
		t.Fatal("expected a non-nil CompiledPlan for a clean bundle")
	}

	groups := map[string]*dslpb.MachineGroup{}
	for _, g := range plan.GetMachineGroups() {
		groups[g.GetName()] = g
	}
	db, ok := groups["db"]
	if !ok {
		t.Fatalf("expected a %q machine group, got %+v", "db", plan.GetMachineGroups())
	}
	if db.GetCount() != 3 {
		t.Fatalf("expected db group count 3, got %d", db.GetCount())
	}
	runner, ok := groups["runner"]
	if !ok {
		t.Fatalf("expected a %q machine group, got %+v", "runner", plan.GetMachineGroups())
	}
	if runner.GetCount() != 1 {
		t.Fatalf("expected runner group count 1, got %d", runner.GetCount())
	}
	if len(plan.GetServices()) == 0 {
		t.Fatal("expected at least one resolved service in the plan")
	}
}

func TestPreviewBrokenBundleReturnsDiagnosticsNilPlan(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)
	delete(files, "providers/yandex/manifest.yaml")

	resp, err := svc.Preview(context.Background(), &dslpb.PreviewRequest{Files: files})
	if err != nil {
		t.Fatalf("Preview must never return an RPC error for a bundle-content problem, got: %v", err)
	}
	if len(resp.GetDiagnostics()) == 0 {
		t.Fatal("expected at least one diagnostic for a missing provider manifest")
	}
	if resp.GetPlan() != nil {
		t.Fatalf("expected a nil plan when compile has error diagnostics, got %+v", resp.GetPlan())
	}
}

func TestComposedSchemaContainsPlatformID(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	resp, err := svc.ComposedSchema(context.Background(), &dslpb.ComposedSchemaRequest{Files: files})
	if err != nil {
		t.Fatalf("ComposedSchema returned an RPC error: %v", err)
	}
	if !strings.Contains(resp.GetSchemaJson(), "platform_id") {
		t.Fatalf("expected schema_json to contain the yandex provider's %q field, got:\n%s", "platform_id", resp.GetSchemaJson())
	}
}

// TestCheckRejectsPathTraversalInProviderModule guards against the
// deriveProviderSchema temp-dir write in service.go writing bundle bytes to
// an attacker-chosen absolute path. A malicious "files" key under
// providers/<name>/module/ containing ".." segments must never let bytes
// land outside the request-scoped temp dir, and Check must surface the
// rejection as a diagnostic rather than silently dropping it (or, worse,
// silently writing the file).
func TestCheckRejectsPathTraversalInProviderModule(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	// escapeDir is a scratch directory distinct from any temp dir
	// deriveProviderSchema itself creates (os.MkdirTemp("", "dsl-provider-module-*")),
	// so if the write escapes its intended temp dir, evidence lands here.
	escapeDir := t.TempDir()
	escapeTarget := filepath.Join(escapeDir, "pwned-marker")

	// Enough "../" segments to walk past root regardless of the actual depth
	// of os.MkdirTemp's directory, then back down into escapeTarget — mirrors
	// "providers/yandex/module/../../../../etc/cron.d/evil" from the report,
	// just pointed at a location this test can assert on afterwards.
	climb := strings.Repeat("../", 30)
	escapeRel := climb + strings.TrimPrefix(filepath.ToSlash(escapeTarget), "/")
	maliciousKey := "providers/yandex/module/" + escapeRel
	files[maliciousKey] = []byte("path traversal payload\n")

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check must never return an RPC error for a bundle-content problem, got: %v", err)
	}

	if _, statErr := os.Stat(escapeTarget); statErr == nil {
		t.Fatalf("path traversal escaped the temp dir: a file was written at %q", escapeTarget)
	}

	var found *dslpb.Diagnostic
	for _, d := range resp.GetDiagnostics() {
		if d.GetModule() == "yandex" && d.GetSeverity() == dslpb.Severity_SEVERITY_ERROR &&
			strings.Contains(strings.ToLower(d.GetMessage()), "travers") {
			found = d
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an ERROR diagnostic rejecting the path-traversal module key, got %+v", resp.GetDiagnostics())
	}
}

func TestCheckMissingProviderManifestIsDiagnosticNotError(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)
	delete(files, "providers/yandex/manifest.yaml")

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check must never return an RPC error for a bundle-content problem, got: %v", err)
	}
	if len(resp.GetDiagnostics()) == 0 {
		t.Fatal("expected at least one diagnostic for a missing provider manifest")
	}
	var found bool
	for _, d := range resp.GetDiagnostics() {
		if d.GetModule() == "yandex" && strings.Contains(d.GetMessage(), "manifest") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a diagnostic naming the missing yandex manifest, got %+v", resp.GetDiagnostics())
	}
}
