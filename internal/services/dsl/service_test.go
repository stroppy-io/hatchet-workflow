package dsl

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
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

// TestComposedSchemaContainsProviderParamField replaces the pre-Task-5
// TestComposedSchemaContainsPlatformID: that test asserted "platform_id"
// (the yandex module's stroppy_machine_ext variable, i.e. the per-machine
// extension block schema — core.schema.json's $defs.machineExt) appeared in
// schema_json, because the old jsonschema-shaped output embedded the whole
// composed document (cluster shape, ext included) verbatim. The new
// schemapb form schema (this task) is deliberately narrower — only
// inputs+params, nested under "provider" (schema.ComposeFormSchema) — ext
// belongs to a different UI concern (validating cluster.yaml's per-machine
// blocks, not the launch form) and is intentionally not part of it, so
// "platform_id" no longer appears. "zone" (postgres-ha's yandex module's
// other, non-ext variable.tf entry — see providers/yandex/module/
// variables.tf) is the right replacement assertion: it IS a params field.
func TestComposedSchemaContainsProviderParamField(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	resp, err := svc.ComposedSchema(context.Background(), &dslpb.ComposedSchemaRequest{Files: files})
	if err != nil {
		t.Fatalf("ComposedSchema returned an RPC error: %v", err)
	}
	if !strings.Contains(resp.GetSchemaJson(), "zone") {
		t.Fatalf("expected schema_json to contain the yandex provider's %q param field, got:\n%s", "zone", resp.GetSchemaJson())
	}
}

// TestComposedSchema_ReturnsSchemapbProtojson verifies ComposedSchema's
// SchemaJson is a schemapb.Schema protojson document (Task 5) rather than
// the JSON-Schema document it returned before this task: it must
// protojson.Unmarshal cleanly into a schemapb.Schema with a non-empty name
// (schema.ComposeFormSchema always names the composed form schema "form" —
// see form.go's NewSchema(namespace, "form", "1") call).
func TestComposedSchema_ReturnsSchemapbProtojson(t *testing.T) {
	svc := NewDslService()
	files := loadBundle(t, postgresHADir)

	resp, err := svc.ComposedSchema(context.Background(), &dslpb.ComposedSchemaRequest{Files: files})
	require.NoError(t, err)

	var s schemapb.Schema
	require.NoError(t, protojson.Unmarshal([]byte(resp.GetSchemaJson()), &s),
		"response must be schemapb.Schema protojson")
	require.NotEmpty(t, s.GetId().GetName(), "composed form schema must carry a non-empty name")
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

// TestCheckDockerBuiltinNeedsNoManifest verifies the docker builtin provider
// compiles with no user-authored providers/docker/manifest.yaml — resolveProvider
// supplies a built-in manifest, so a bare docker recipe is not rejected with
// "manifest not found".
// dockerBundleFiles returns a minimal, self-contained docker-builtin recipe
// bundle (no provider directory — see resolveProvider's BuiltinDockerManifest
// path). Shared by TestCheckDockerBuiltinNeedsNoManifest and (Task 7)
// TestCompileBundle_LegacySignatureUnaffected, which both need a bundle that
// compiles clean with zero providers/<name>/ files.
func dockerBundleFiles() map[string][]byte {
	return map[string][]byte{
		"cluster.yaml": []byte("version: 1\n" +
			"provider:\n  use: docker\n" +
			"machines:\n  db:\n    count: 1\n    resources: { cpu: 2, ram: 2g, disk: { size: 10g, type: ssd } }\n" +
			"services:\n  postgres:\n    on: db\n    image: postgres:17\n    network: host\n"),
		"workflow.yaml": []byte("jobs:\n  postgres:\n    service: postgres\n"),
	}
}

func TestCheckDockerBuiltinNeedsNoManifest(t *testing.T) {
	svc := NewDslService()
	files := dockerBundleFiles()

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check must not RPC-error for a docker bundle, got: %v", err)
	}
	for _, d := range resp.GetDiagnostics() {
		if d.GetSeverity() == dslpb.Severity_SEVERITY_ERROR {
			t.Fatalf("docker builtin bundle must compile clean, got error diagnostic: %+v", d)
		}
	}
}

// TestComposedSchemaDockerBuiltinNoProviderField exercises the nil-params
// path ComposedSchema takes for the docker builtin (no providers/ directory
// at all in the bundle, so resolveProviderName returns "" and params stays
// nil -- see service.go's ComposedSchema doc comment). This is the live
// docker recipe-run path, so its form schema must still compose cleanly: the
// workflow's declared top-level "inputs:" hoisted to the form's fields, and
// no "provider" object field at all (schema.ComposeFormSchema only adds one
// when params is non-nil with fields -- see form.go).
func TestComposedSchemaDockerBuiltinNoProviderField(t *testing.T) {
	svc := NewDslService()
	files := map[string][]byte{
		"cluster.yaml": []byte("version: 1\n" +
			"provider:\n  use: docker\n" +
			"machines:\n  db:\n    count: 1\n    resources: { cpu: 2, ram: 2g, disk: { size: 10g, type: ssd } }\n" +
			"services:\n  postgres:\n    on: db\n    image: postgres:17\n    network: host\n"),
		"workflow.yaml": []byte("inputs:\n  iterations: int\n" +
			"jobs:\n  postgres:\n    service: postgres\n"),
	}

	resp, err := svc.ComposedSchema(context.Background(), &dslpb.ComposedSchemaRequest{Files: files})
	require.NoError(t, err, "ComposedSchema must succeed for the nil-params docker builtin path")

	var s schemapb.Schema
	require.NoError(t, protojson.Unmarshal([]byte(resp.GetSchemaJson()), &s),
		"response must be schemapb.Schema protojson")

	byName := map[string]*schemapb.Schema_Filed{}
	for _, f := range s.GetFields() {
		byName[f.GetName()] = f
	}
	require.Contains(t, byName, "iterations", "workflow input must be hoisted to the form's top level")
	require.NotContains(t, byName, "provider", "docker builtin has no tf module: form must carry no provider field")
}

// fakeVersionSource is a test-only VersionSource returning a fixed list,
// standing in for internal/services/stroppy's GitHub-backed one (Task 7).
type fakeVersionSource struct {
	versions []string
	err      error
}

func (f fakeVersionSource) List(context.Context) ([]string, error) {
	return f.versions, f.err
}

// withStroppyImage returns a copy of files with cluster.yaml's stroppy
// service image tag replaced by image (e.g. "stroppy:9.9.9-bogus"),
// standing in for a hand-edited recipe pinning an arbitrary stroppy
// version. postgres-ha's fixture pins "image: stroppy:latest" (see
// examples/dsl/postgres-ha/cluster.yaml).
func withStroppyImage(t *testing.T, files map[string][]byte, image string) map[string][]byte {
	t.Helper()

	out := make(map[string][]byte, len(files))
	for k, v := range files {
		out[k] = v
	}
	cluster, ok := out[clusterFile]
	if !ok {
		t.Fatalf("bundle has no %s", clusterFile)
	}
	patched := bytes.Replace(cluster, []byte("image: stroppy:latest"), []byte("image: "+image), 1)
	if bytes.Equal(patched, cluster) {
		t.Fatalf("expected to patch %s's stroppy image, but %q was not found", clusterFile, "image: stroppy:latest")
	}
	out[clusterFile] = patched
	return out
}

// findDiagnostic returns the first diagnostic whose Module and Message
// (substring, case-insensitive) match, or nil.
func findDiagnostic(diags []*dslpb.Diagnostic, module, messageContains string) *dslpb.Diagnostic {
	for _, d := range diags {
		if d.GetModule() == module && strings.Contains(strings.ToLower(d.GetMessage()), strings.ToLower(messageContains)) {
			return d
		}
	}
	return nil
}

func TestCheckUnknownStroppyVersionWarns(t *testing.T) {
	svc := NewDslService(WithVersionSource(fakeVersionSource{versions: []string{"1.0.0", "2.0.0"}}))
	files := withStroppyImage(t, loadBundle(t, postgresHADir), "stroppy:9.9.9-bogus")

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check must never return an RPC error for a bundle-content problem, got: %v", err)
	}

	d := findDiagnostic(resp.GetDiagnostics(), "stroppy", "9.9.9-bogus")
	if d == nil {
		t.Fatalf("expected a diagnostic naming the unknown stroppy version %q, got %+v", "9.9.9-bogus", resp.GetDiagnostics())
	}
	if d.GetSeverity() != dslpb.Severity_SEVERITY_WARNING {
		t.Fatalf("expected the stroppy-version diagnostic to be a WARNING (advisory only), got %v", d.GetSeverity())
	}
}

func TestCheckKnownStroppyVersionNoWarning(t *testing.T) {
	svc := NewDslService(WithVersionSource(fakeVersionSource{versions: []string{"1.0.0", "2.0.0"}}))
	files := withStroppyImage(t, loadBundle(t, postgresHADir), "stroppy:2.0.0")

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check must never return an RPC error for a bundle-content problem, got: %v", err)
	}

	if d := findDiagnostic(resp.GetDiagnostics(), "stroppy", "not found in known releases"); d != nil {
		t.Fatalf("expected no stroppy-version diagnostic for a known tag, got %+v", d)
	}
}

func TestCheckNoVersionSourceSkipsStroppyValidation(t *testing.T) {
	// No WithVersionSource — the default every pre-Task-7 call site uses.
	// An unknown/bogus tag must not warn: absent a version source, there is
	// nothing to validate against, and the check must not manufacture a
	// false "unknown version" out of that absence.
	svc := NewDslService()
	files := withStroppyImage(t, loadBundle(t, postgresHADir), "stroppy:9.9.9-bogus")

	resp, err := svc.Check(context.Background(), &dslpb.CheckRequest{Files: files})
	if err != nil {
		t.Fatalf("Check must never return an RPC error for a bundle-content problem, got: %v", err)
	}
	if d := findDiagnostic(resp.GetDiagnostics(), "stroppy", "not found in known releases"); d != nil {
		t.Fatalf("expected no stroppy-version diagnostic without an injected VersionSource, got %+v", d)
	}
}

func TestPreviewUnknownStroppyVersionWarns(t *testing.T) {
	svc := NewDslService(WithVersionSource(fakeVersionSource{versions: []string{"1.0.0"}}))
	files := withStroppyImage(t, loadBundle(t, postgresHADir), "stroppy:9.9.9-bogus")

	resp, err := svc.Preview(context.Background(), &dslpb.PreviewRequest{Files: files})
	if err != nil {
		t.Fatalf("Preview must never return an RPC error for a bundle-content problem, got: %v", err)
	}
	if d := findDiagnostic(resp.GetDiagnostics(), "stroppy", "9.9.9-bogus"); d == nil {
		t.Fatalf("expected a diagnostic naming the unknown stroppy version, got %+v", resp.GetDiagnostics())
	}
	// The stroppy-version diagnostic is a warning, not an error, so a
	// bundle that otherwise compiles cleanly must still return a non-nil
	// plan (Preview only nils the plan when diags.HasErrors()).
	if resp.GetPlan() == nil {
		t.Fatal("expected a non-nil plan: the stroppy-version diagnostic is advisory, not an error")
	}
}

// TestDslServiceCheckBundleMethodWarnsOnUnknownStroppyVersion covers the
// (*DslService) CheckBundle method (distinct from the package-level
// CheckBundle function, which has no VersionSource and never warns): it is
// bound as internal/services/recipe.Deps.Checker in production
// (internal/app/run.go) so the stored-recipe Create/CheckRecipe path gets the
// same advisory stroppy-version diagnostic DslService.Check gives the live
// editor.
func TestDslServiceCheckBundleMethodWarnsOnUnknownStroppyVersion(t *testing.T) {
	svc := NewDslService(WithVersionSource(fakeVersionSource{versions: []string{"1.0.0", "2.0.0"}}))
	files := withStroppyImage(t, loadBundle(t, postgresHADir), "stroppy:9.9.9-bogus")

	diags, err := svc.CheckBundle(context.Background(), files)
	if err != nil {
		t.Fatalf("CheckBundle must never return a Go error for a bundle-content problem, got: %v", err)
	}
	if d := findDiagnostic(diags, "stroppy", "9.9.9-bogus"); d == nil {
		t.Fatalf("expected a diagnostic naming the unknown stroppy version, got %+v", diags)
	}
}

// TestDslServiceCheckBundleMethodNoVersionSourceSkipsValidation mirrors
// TestCheckNoVersionSourceSkipsStroppyValidation for the method form: absent
// an injected VersionSource, the method behaves exactly like the
// package-level CheckBundle function — no manufactured warning.
func TestDslServiceCheckBundleMethodNoVersionSourceSkipsValidation(t *testing.T) {
	svc := NewDslService()
	files := withStroppyImage(t, loadBundle(t, postgresHADir), "stroppy:9.9.9-bogus")

	diags, err := svc.CheckBundle(context.Background(), files)
	if err != nil {
		t.Fatalf("CheckBundle must never return a Go error for a bundle-content problem, got: %v", err)
	}
	if d := findDiagnostic(diags, "stroppy", "not found in known releases"); d != nil {
		t.Fatalf("expected no stroppy-version diagnostic without an injected VersionSource, got %+v", d)
	}
}

// fakeProviderResolver is a test-only dsl.ProviderResolver standing in for
// catalog.CatalogProviderResolver (Task 7): it records the (slug, version)
// it was called with and returns a fixed (files, version, err).
type fakeProviderResolver struct {
	files      map[string][]byte
	version    uint32
	err        error
	gotSlug    string
	gotVersion uint32
}

func (f *fakeProviderResolver) ResolveProvider(_ context.Context, _, slug string, version uint32) (map[string][]byte, uint32, error) {
	f.gotSlug, f.gotVersion = slug, version
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.files, f.version, nil
}

// hasWarning reports whether diags contains a Warning-severity diagnostic
// whose message contains substr (case-insensitive).
func hasWarning(diags diag.List, substr string) bool {
	for _, d := range diags {
		if d.Severity == diag.Warning && strings.Contains(strings.ToLower(d.Message), strings.ToLower(substr)) {
			return true
		}
	}
	return false
}

func TestCompileBundleWithCatalog_PinnedVersionParsed(t *testing.T) {
	resolver := &fakeProviderResolver{
		files:   map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
		version: 3,
	}
	files := map[string][]byte{
		"cluster.yaml":  []byte("version: 1\nprovider:\n  use: yandex@3\nmachines:\n  db:\n    count: 1\nservices: {}\n"),
		"workflow.yaml": []byte("jobs: {}\n"),
	}
	_, diags := CompileBundleWithCatalog(context.Background(), "tenant-1", files, resolver)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.String())
	}
	if resolver.gotSlug != "yandex" || resolver.gotVersion != 3 {
		t.Fatalf("resolver called with (%q, %d), want (yandex, 3)", resolver.gotSlug, resolver.gotVersion)
	}
}

func TestCompileBundleWithCatalog_UnpinnedWarnsAndResolvesLatest(t *testing.T) {
	resolver := &fakeProviderResolver{
		files:   map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
		version: 5,
	}
	files := map[string][]byte{
		"cluster.yaml":  []byte("version: 1\nprovider:\n  use: yandex\nmachines:\n  db:\n    count: 1\nservices: {}\n"),
		"workflow.yaml": []byte("jobs: {}\n"),
	}
	_, diags := CompileBundleWithCatalog(context.Background(), "tenant-1", files, resolver)
	if resolver.gotVersion != 0 {
		t.Fatalf("unpinned use should request version=0 (latest), got %d", resolver.gotVersion)
	}
	if !hasWarning(diags, "pin") {
		t.Fatalf("expected a reproducibility warning diagnostic, got: %s", diags.String())
	}
}

func TestCompileBundle_LegacySignatureUnaffected(t *testing.T) {
	// Zero-arg CompileBundle must keep working exactly as before this task —
	// no resolver, no tenant, same-bundle providers/<name>/ lookup.
	files := dockerBundleFiles()
	_, diags := CompileBundle(files)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %s", diags.String())
	}
}
