package dsl_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
)

// update, when passed as `-update`, (re)writes every golden file a test in
// this package compares against instead of failing on a mismatch — the
// standard Go golden-file pattern.
var update = flag.Bool("update", false, "update golden files")

const postgresHADir = "../../examples/dsl/postgres-ha"

// loadDir walks dir recursively and returns every regular file's contents
// keyed by its slash path relative to dir (e.g. "components/etcd/component.yaml"),
// matching the shape include.Sources.Files expects and the way
// compiler_test.go's own inline fixtures key their Files map.
func loadDir(t *testing.T, dir string) include.Sources {
	t.Helper()

	files := map[string][]byte{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(path) //nolint:gosec // test-only, reading our own fixture tree.
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		t.Fatalf("loadDir(%q): %v", dir, err)
	}
	return include.Sources{Files: files}
}

// compareOrUpdateGolden implements the standard Go golden-file pattern: with
// -update it (re)writes path (creating parent directories as needed);
// otherwise it reads path and fails with both dumps side by side if got
// differs.
func compareOrUpdateGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // test-only golden dir.
			t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil { //nolint:gosec // test-only golden file.
			t.Fatalf("write golden %q: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path) //nolint:gosec // test-only golden file.
	if err != nil {
		t.Fatalf("read golden %q: %v (run with -update to create it)", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf(
			"golden mismatch for %s (run `go test ./internal/dsl/ -run PostgresHAGolden -update` to refresh if this is intentional)\n--- want ---\n%s\n--- got ---\n%s",
			path, want, got,
		)
	}
}

// normalizeJSON re-encodes protojson output through encoding/json's
// map[string]any round-trip: encoding/json.Marshal sorts map keys
// alphabetically (guaranteed by the stdlib, unlike protojson's own field
// ordering, which is explicitly documented as unstable across releases/
// calls), so two protojson.Marshal calls over proto.Equal messages that
// happen to differ in incidental whitespace/key order both collapse to the
// exact same normalized bytes.
func normalizeJSON(t *testing.T, raw []byte) []byte {
	t.Helper()

	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal protojson output: %v", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal normalized json: %v", err)
	}
	out = append(out, '\n')
	return out
}

// buildPostgresHAInput loads the postgres-ha example bundle and its yandex
// provider's derived schema/manifest, producing the dsl.Input the golden
// tests compile.
func buildPostgresHAInput(t *testing.T) dsl.Input {
	t.Helper()

	src := loadDir(t, postgresHADir)

	params, ext, err := schema.DeriveParamsSchema(postgresHADir + "/providers/yandex/module")
	if err != nil {
		t.Fatalf("DeriveParamsSchema: %v", err)
	}
	composed, _, err := schema.Compose(map[string]schema.ProviderSchemas{"yandex": {Params: params, Ext: ext}}, nil)
	if err != nil {
		t.Fatalf("schema.Compose: %v", err)
	}

	manifestSrc, ok := src.Files["providers/yandex/manifest.yaml"]
	if !ok {
		t.Fatal("providers/yandex/manifest.yaml missing from loaded bundle")
	}
	manifest, manifestDiags := ast.DecodeProviderManifest("providers/yandex/manifest.yaml", manifestSrc)
	if manifestDiags.HasErrors() {
		t.Fatalf("DecodeProviderManifest: %+v", manifestDiags)
	}

	return dsl.Input{
		Sources:  src,
		Provider: manifest,
		Composed: composed,
	}
}

// TestPostgresHAGolden compiles the postgres-ha example recipe end to end
// and compares its normalized protojson plan against
// testdata/postgres-ha.golden.json (refreshed via `-update`).
func TestPostgresHAGolden(t *testing.T) {
	in := buildPostgresHAInput(t)

	plan, diags := dsl.Compile(in)
	if diags.HasErrors() {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if plan == nil {
		t.Fatal("expected a non-nil plan")
	}

	raw, err := protojson.MarshalOptions{Multiline: true}.Marshal(plan)
	if err != nil {
		t.Fatalf("protojson.Marshal: %v", err)
	}

	got := normalizeJSON(t, raw)
	compareOrUpdateGolden(t, "testdata/postgres-ha.golden.json", got)
}

// TestPostgresHAGoldenStable proves the determinism claim the golden test
// depends on: compiling and normalizing the same bundle twice in the same
// process produces byte-identical output, independent of whatever
// nondeterminism protojson.Marshal itself might introduce across calls.
func TestPostgresHAGoldenStable(t *testing.T) {
	in := buildPostgresHAInput(t)

	plan1, diags1 := dsl.Compile(in)
	if diags1.HasErrors() {
		t.Fatalf("unexpected diags (first run): %+v", diags1)
	}
	plan2, diags2 := dsl.Compile(in)
	if diags2.HasErrors() {
		t.Fatalf("unexpected diags (second run): %+v", diags2)
	}

	raw1, err := protojson.MarshalOptions{Multiline: true}.Marshal(plan1)
	if err != nil {
		t.Fatalf("protojson.Marshal (first run): %v", err)
	}
	raw2, err := protojson.MarshalOptions{Multiline: true}.Marshal(plan2)
	if err != nil {
		t.Fatalf("protojson.Marshal (second run): %v", err)
	}

	got1 := normalizeJSON(t, raw1)
	got2 := normalizeJSON(t, raw2)
	if !bytes.Equal(got1, got2) {
		t.Fatalf("normalized plan differs across two Compile calls:\n1: %s\n2: %s", got1, got2)
	}
}

// TestPostgresHAContractViolation locks the negative smoke case the brief
// calls out explicitly: breaking the db group's count down to 2 (an even
// number, below etcd's quorum floor) must surface the etcd component's own
// quorum diagnostic, patched in-memory into the loaded bundle rather than by
// editing the on-disk fixture.
func TestPostgresHAContractViolation(t *testing.T) {
	in := buildPostgresHAInput(t)

	clusterSrc, ok := in.Sources.Files["cluster.yaml"]
	if !ok {
		t.Fatal("cluster.yaml missing from loaded bundle")
	}
	const original = "count: 3"
	const broken = "count: 2"
	if !bytes.Contains(clusterSrc, []byte(original)) {
		t.Fatalf("cluster.yaml fixture no longer contains %q — update this test's patch", original)
	}
	patched := bytes.Replace(clusterSrc, []byte(original), []byte(broken), 1)

	patchedFiles := make(map[string][]byte, len(in.Sources.Files))
	for k, v := range in.Sources.Files {
		patchedFiles[k] = v
	}
	patchedFiles["cluster.yaml"] = patched
	in.Sources = include.Sources{Files: patchedFiles}

	plan, diags := dsl.Compile(in)
	if plan != nil {
		t.Fatalf("expected a nil plan on contract violation, got: %+v", plan)
	}
	if !diags.HasErrors() {
		t.Fatal("expected a contract violation diagnostic")
	}

	found := false
	for _, d := range diags {
		if d.Module == "etcd" && strings.Contains(d.Message, "кворум") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a diag with Module=%q about the quorum, got: %+v", "etcd", diags)
	}
}
