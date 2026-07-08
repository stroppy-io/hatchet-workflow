package catalog

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

func TestDeriveWorkflowSummary_ReadsClusterYAML(t *testing.T) {
	files := map[string][]byte{
		"cluster.yaml": []byte("version: 1\nprovider:\n  use: yandex\nmachines:\n  db:\n    count: 1\nservices:\n  postgres: {}\n"),
	}
	summary := DeriveWorkflowSummary(files, nil)
	if summary.GetProviderSlug() != "yandex" {
		t.Fatalf("provider_slug = %q, want yandex", summary.GetProviderSlug())
	}
	if summary.GetMachineGroupCount() != 1 || summary.GetServiceCount() != 1 {
		t.Fatalf("counts = (%d,%d), want (1,1)", summary.GetMachineGroupCount(), summary.GetServiceCount())
	}
	if !summary.GetCompiles() {
		t.Fatal("compiles should be true when diags is empty")
	}
}

func TestDeriveWorkflowSummary_MissingClusterYAML(t *testing.T) {
	summary := DeriveWorkflowSummary(map[string][]byte{}, nil)
	if summary.GetProviderSlug() != "" || summary.GetMachineGroupCount() != 0 || summary.GetServiceCount() != 0 {
		t.Fatalf("expected zero-value summary, got %+v", summary)
	}
	if !summary.GetCompiles() {
		t.Fatal("compiles should be true when diags is empty, even without cluster.yaml")
	}
}

func TestDeriveWorkflowSummary_CompilesFalseWithDiags(t *testing.T) {
	summary := DeriveWorkflowSummary(map[string][]byte{}, []*dslpb.Diagnostic{{Message: "boom"}})
	if summary.GetCompiles() {
		t.Fatal("compiles should be false when diags is non-empty")
	}
}

func TestDeriveProviderSummary_ReadsManifest(t *testing.T) {
	manifest, diags := ast.DecodeProviderManifest("providers/yandex/manifest.yaml", []byte("name: yandex\nprovides:\n  - machines\n  - network\n"))
	if diags.HasErrors() {
		t.Fatalf("decode manifest: %s", diags.String())
	}
	summary := DeriveProviderSummary(manifest)
	if len(summary.GetProvides()) != 2 {
		t.Fatalf("provides = %v, want 2 entries", summary.GetProvides())
	}
	_ = catalogpb.Kind_KIND_PROVIDER
	_ = dslpb.Diagnostic{}
}

func TestDeriveProviderSummary_NilManifest(t *testing.T) {
	summary := DeriveProviderSummary(nil)
	if len(summary.GetProvides()) != 0 {
		t.Fatalf("expected empty provides, got %v", summary.GetProvides())
	}
}
