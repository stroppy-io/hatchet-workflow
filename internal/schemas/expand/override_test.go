package expand

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/schemapb/schemapb"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/providers"
)

func ovStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("struct: %v", err)
	}
	return s
}

func TestOverrideYandexNarrowsToTenant(t *testing.T) {
	tenant := map[string]any{
		providers.FieldDiskTypes:  []any{"network-ssd", "network-ssd-io-m3"},
		providers.FieldZones:      []any{"ru-central1-a", "ru-central1-b"},
		providers.FieldPlatformID: "standard-v3",
	}
	s := OverrideSchema(ProviderYandex, "postgres", "database", tenant)

	// in-catalog values validate clean.
	ok := ovStruct(t, map[string]any{"disk_type": "network-ssd-io-m3", "zone": "ru-central1-b"})
	for _, e := range s.ValidateStruct(ok) {
		if e.GetSeverity() == schemapb.Schema_Filed_ERROR {
			t.Errorf("unexpected error: %s %s", e.GetField(), e.GetMessage())
		}
	}

	// a disk class NOT in the tenant catalog is rejected.
	bad := ovStruct(t, map[string]any{"disk_type": "network-hdd"})
	found := false
	for _, e := range s.ValidateStruct(bad) {
		if e.GetField() == "disk_type" && e.GetSeverity() == schemapb.Schema_Filed_ERROR {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected disk_type out-of-catalog error")
	}
}

func TestOverrideDockerEmpty(t *testing.T) {
	s := OverrideSchema(ProviderDocker, "postgres", "database", nil)
	if len(s.GetFields()) != 0 {
		t.Fatalf("docker override should have no fields, got %d", len(s.GetFields()))
	}
}
