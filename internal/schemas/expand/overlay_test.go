package expand

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/providers"
)

func TestApplyYandexOverlay(t *testing.T) {
	prov := map[string]any{
		providers.FieldPlatformID:                 "standard-v3",
		providers.FieldZones:                      []any{"ru-central1-a", "ru-central1-b", "ru-central1-d"},
		providers.FieldDiskTypes:                  []any{"network-ssd", "network-ssd-io-m3"},
		providers.FieldSoftwareAcceleratedNetwork: true,
	}
	vms := []VM{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}
	out := ApplyProvider(ProviderYandex, prov, vms)

	wantZones := []string{"ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-a"}
	for i, vm := range out {
		if vm.Provider.Platform != "standard-v3" {
			t.Errorf("vm %d: platform %q", i, vm.Provider.Platform)
		}
		if vm.Provider.DiskType != "network-ssd" {
			t.Errorf("vm %d: disk %q", i, vm.Provider.DiskType)
		}
		if vm.Provider.NetworkAccel != "software_accelerated" {
			t.Errorf("vm %d: accel %q", i, vm.Provider.NetworkAccel)
		}
		if vm.Provider.Zone != wantZones[i] {
			t.Errorf("vm %d: zone %q want %q", i, vm.Provider.Zone, wantZones[i])
		}
	}
}

func TestApplyDockerNoOverlay(t *testing.T) {
	vms := []VM{{Name: "a"}}
	out := ApplyProvider(ProviderDocker, nil, vms)
	if out[0].Provider != (ProviderParams{}) {
		t.Fatalf("docker should set no provider params, got %+v", out[0].Provider)
	}
}
