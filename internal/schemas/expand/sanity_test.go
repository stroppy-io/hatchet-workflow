package expand

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/providers"
)

func TestYandexCapacity(t *testing.T) {
	prov := map[string]any{providers.FieldLimits: map[string]any{
		providers.FieldMaxCoresPerNode: float64(4),
		providers.FieldMaxNodes:        float64(2),
	}}
	// vm cores 8 > 4, and 3 nodes > 2 → 2 errors.
	vms := []VM{{Name: "a", Shape: Shape{Cores: 8}}, {Name: "b", Shape: Shape{Cores: 2}}, {Name: "c", Shape: Shape{Cores: 2}}}
	if errs := Sanity("postgres", nil, ProviderYandex, prov, vms); len(errs) != 2 {
		t.Fatalf("want 2 errors, got %d: %v", len(errs), errs)
	}
	// within limits → none.
	ok := []VM{{Name: "a", Shape: Shape{Cores: 4}}}
	if errs := Sanity("postgres", nil, ProviderYandex, prov, ok); len(errs) != 0 {
		t.Fatalf("want 0 errors, got %v", errs)
	}
}

func TestDockerCapacity(t *testing.T) {
	prov := map[string]any{providers.FieldLimits: map[string]any{
		providers.FieldMaxCores: float64(4),
	}}
	vms := []VM{{Shape: Shape{Cores: 3}}, {Shape: Shape{Cores: 3}}} // sum 6 > 4
	if errs := Sanity("postgres", nil, ProviderDocker, prov, vms); len(errs) != 1 {
		t.Fatalf("want 1 error, got %v", errs)
	}
}

func TestMirror3DCZones(t *testing.T) {
	db := map[string]any{ydbStorageKey: map[string]any{ydbFaultKey: "mirror-3-dc"}}

	// yandex, 2 zones → error.
	prov2 := map[string]any{providers.FieldZones: []any{"a", "b"}}
	if errs := Sanity("ydb", db, ProviderYandex, prov2, nil); len(errs) != 1 {
		t.Fatalf("2 zones: want 1 error, got %v", errs)
	}
	// yandex, 3 zones → ok.
	prov3 := map[string]any{providers.FieldZones: []any{"a", "b", "c"}}
	if errs := Sanity("ydb", db, ProviderYandex, prov3, nil); len(errs) != 0 {
		t.Fatalf("3 zones: want 0 errors, got %v", errs)
	}
	// docker → error (no zones).
	if errs := Sanity("ydb", db, ProviderDocker, nil, nil); len(errs) != 1 {
		t.Fatalf("docker: want 1 error, got %v", errs)
	}
	// non-mirror-3-dc → no check.
	if errs := Sanity("ydb", map[string]any{}, ProviderDocker, nil, nil); len(errs) != 0 {
		t.Fatalf("no mirror: want 0 errors, got %v", errs)
	}
}

// storage / fault_tolerance field tokens (match the ydb schema).
const (
	ydbStorageKey = "storage"
	ydbFaultKey   = "fault_tolerance"
)
