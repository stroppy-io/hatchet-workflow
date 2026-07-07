package quotas

import (
	"errors"
	"testing"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// --- amountsForGroups: pure per-kind sum -------------------------------

func TestAmountsForGroupsSumsCpuRamDiskAcrossGroups(t *testing.T) {
	groups := []*dslpb.MachineGroup{
		{
			Name: "db", Count: 3, Cpu: 4, RamMb: 8192,
			Disks: []*dslpb.DiskSpec{{SizeGb: 50}, {SizeGb: 20}},
		},
		{
			Name: "runner", Count: 1, Cpu: 2, RamMb: 2048,
			Disks: []*dslpb.DiskSpec{{SizeGb: 10}},
		},
	}
	amounts := amountsForGroups(groups)

	if got, want := amounts[quotaKindCPU], uint64(3*4+1*2); got != want {
		t.Errorf("cpu = %d, want %d", got, want)
	}
	if got, want := amounts[quotaKindRAM], uint64(3*8192+1*2048); got != want {
		t.Errorf("ram_mb = %d, want %d", got, want)
	}
	if got, want := amounts[quotaKindDisk], uint64(3*(50+20)+1*10); got != want {
		t.Errorf("disk_gb = %d, want %d", got, want)
	}
}

func TestAmountsForGroupsSkipsZeroCountGroup(t *testing.T) {
	groups := []*dslpb.MachineGroup{
		{Name: "empty", Count: 0, Cpu: 100, RamMb: 100000, Disks: []*dslpb.DiskSpec{{SizeGb: 1000}}},
	}
	amounts := amountsForGroups(groups)
	if amounts[quotaKindCPU] != 0 || amounts[quotaKindRAM] != 0 || amounts[quotaKindDisk] != 0 {
		t.Fatalf("zero-count group contributed demand: %+v", amounts)
	}
}

func TestAmountsForGroupsEmptyGroupsAllZero(t *testing.T) {
	amounts := amountsForGroups(nil)
	if amounts[quotaKindCPU] != 0 || amounts[quotaKindRAM] != 0 || amounts[quotaKindDisk] != 0 {
		t.Fatalf("empty groups produced non-zero demand: %+v", amounts)
	}
}

// --- buildQuotaAmounts: per-provider quota_name mapping + unit conversion --

func TestBuildQuotaAmountsDockerUsesDockerSourceQuotaNamesNoConversion(t *testing.T) {
	groups := []*dslpb.MachineGroup{
		{Name: "app", Count: 2, Cpu: 2, RamMb: 4096, Disks: []*dslpb.DiskSpec{{SizeGb: 30}}},
	}
	amounts, err := buildQuotaAmounts(deploymentpb.Provider_PROVIDER_DOCKER, groups)
	if err != nil {
		t.Fatalf("buildQuotaAmounts: %v", err)
	}
	byName := map[string]QuotaAmount{}
	for _, a := range amounts {
		byName[a.QuotaName] = a
	}
	if got, want := len(amounts), 3; got != want {
		t.Fatalf("amounts count = %d, want %d: %+v", got, want, amounts)
	}
	if a, ok := byName["host.cpuCores"]; !ok || a.Amount != 4 || a.Units != "cores" {
		t.Errorf("host.cpuCores = %+v, want amount=4 units=cores", a)
	}
	if a, ok := byName["host.memory.size"]; !ok || a.Amount != 8192 || a.Units != "MiB" {
		t.Errorf("host.memory.size = %+v, want amount=8192 units=MiB (no MB->MiB conversion)", a)
	}
	if a, ok := byName["host.disk.size"]; !ok || a.Amount != 60 || a.Units != "GiB" {
		t.Errorf("host.disk.size = %+v, want amount=60 units=GiB", a)
	}
}

func TestBuildQuotaAmountsYandexConvertsRamMbToGbAndUsesSsdDiskQuota(t *testing.T) {
	groups := []*dslpb.MachineGroup{
		{Name: "db", Count: 1, Cpu: 8, RamMb: 16384, Disks: []*dslpb.DiskSpec{{SizeGb: 100, Type: "network-ssd"}}},
	}
	amounts, err := buildQuotaAmounts(deploymentpb.Provider_PROVIDER_YANDEX, groups)
	if err != nil {
		t.Fatalf("buildQuotaAmounts: %v", err)
	}
	byName := map[string]QuotaAmount{}
	for _, a := range amounts {
		byName[a.QuotaName] = a
	}
	if a, ok := byName["compute.instanceCores.count"]; !ok || a.Amount != 8 || a.Units != "count" {
		t.Errorf("compute.instanceCores.count = %+v, want amount=8 units=count", a)
	}
	// 16384 MB / 1024 = 16 GB exactly.
	if a, ok := byName["compute.instanceMemory.size"]; !ok || a.Amount != 16 || a.Units != "GB" {
		t.Errorf("compute.instanceMemory.size = %+v, want amount=16 units=GB", a)
	}
	if a, ok := byName["compute.ssdDisks.size"]; !ok || a.Amount != 100 || a.Units != "GB" {
		t.Errorf("compute.ssdDisks.size = %+v, want amount=100 units=GB", a)
	}
}

func TestBuildQuotaAmountsYandexRoundsRamMbUpToNextGb(t *testing.T) {
	groups := []*dslpb.MachineGroup{{Name: "db", Count: 1, Cpu: 1, RamMb: 1025}}
	amounts, err := buildQuotaAmounts(deploymentpb.Provider_PROVIDER_YANDEX, groups)
	if err != nil {
		t.Fatalf("buildQuotaAmounts: %v", err)
	}
	for _, a := range amounts {
		if a.QuotaName == "compute.instanceMemory.size" {
			if a.Amount != 2 {
				t.Errorf("1025 MB rounded to %d GB, want 2 (round up, never under-reserve)", a.Amount)
			}
			return
		}
	}
	t.Fatal("compute.instanceMemory.size amount missing")
}

func TestBuildQuotaAmountsDropsZeroDimensions(t *testing.T) {
	groups := []*dslpb.MachineGroup{{Name: "app", Count: 1, Cpu: 1, RamMb: 512}} // no disks
	amounts, err := buildQuotaAmounts(deploymentpb.Provider_PROVIDER_DOCKER, groups)
	if err != nil {
		t.Fatalf("buildQuotaAmounts: %v", err)
	}
	for _, a := range amounts {
		if a.QuotaName == "host.disk.size" {
			t.Fatalf("expected no disk reservation row for a diskless group, got %+v", a)
		}
	}
}

func TestBuildQuotaAmountsUnsupportedProviderErrors(t *testing.T) {
	_, err := buildQuotaAmounts(deploymentpb.Provider_PROVIDER_UNSPECIFIED, []*dslpb.MachineGroup{{Count: 1, Cpu: 1}})
	if err == nil {
		t.Fatal("expected an error for an unsupported provider")
	}
}

// --- providerFromDslName ------------------------------------------------

func TestProviderFromDslName(t *testing.T) {
	cases := map[string]deploymentpb.Provider{
		"docker": deploymentpb.Provider_PROVIDER_DOCKER,
		"yandex": deploymentpb.Provider_PROVIDER_YANDEX,
		"":       deploymentpb.Provider_PROVIDER_UNSPECIFIED,
		"aws":    deploymentpb.Provider_PROVIDER_UNSPECIFIED,
	}
	for name, want := range cases {
		if got := providerFromDslName(name); got != want {
			t.Errorf("providerFromDslName(%q) = %s, want %s", name, got, want)
		}
	}
}

// --- servicesForAmounts ---------------------------------------------------

func TestServicesForAmountsExtractsServicePrefix(t *testing.T) {
	got := servicesForAmounts([]QuotaAmount{
		{QuotaName: "compute.instanceCores.count"},
		{QuotaName: "compute.instanceMemory.size"},
		{QuotaName: "host.cpuCores"},
	})
	want := []string{"compute", "host"}
	if len(got) != len(want) {
		t.Fatalf("services = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("services = %v, want %v", got, want)
		}
	}
}

// --- checkAvailable: the over-limit decision, pure (no DB) ---------------

func TestCheckAvailableUnderLimitPasses(t *testing.T) {
	if err := checkAvailable("host.cpuCores", 64, 10, 20); err != nil {
		t.Fatalf("checkAvailable: unexpected error: %v", err)
	}
}

func TestCheckAvailableExactlyAtLimitPasses(t *testing.T) {
	if err := checkAvailable("host.cpuCores", 64, 44, 20); err != nil {
		t.Fatalf("checkAvailable: unexpected error at the exact limit: %v", err)
	}
}

func TestCheckAvailableOverLimitErrors(t *testing.T) {
	err := checkAvailable("host.cpuCores", 64, 50, 20)
	if err == nil {
		t.Fatal("expected an over-limit error")
	}
	if !errors.Is(err, ErrInsufficient) {
		t.Fatalf("error = %v, want it to wrap ErrInsufficient", err)
	}
}

// --- Manager.Reserve: validation that never touches the store -----------

func TestManagerReserveRequiresTenantID(t *testing.T) {
	m := &Manager{cfg: ManagerConfig{}}
	err := m.Reserve(nil, "", "run-1", "wf-1", "docker", nil) //nolint:staticcheck // nil ctx: this path returns before ctx is used.
	if err == nil {
		t.Fatal("expected an error for an empty tenant_id")
	}
}

func TestManagerReserveRequiresRunID(t *testing.T) {
	m := &Manager{cfg: ManagerConfig{}}
	err := m.Reserve(nil, "tenant-1", "", "wf-1", "docker", nil) //nolint:staticcheck // nil ctx: this path returns before ctx is used.
	if err == nil {
		t.Fatal("expected an error for an empty run_id")
	}
}

func TestManagerReserveUnsupportedProviderErrorsBeforeTouchingStore(t *testing.T) {
	// m.store is nil: if Reserve ever dereferenced it before returning
	// buildQuotaAmounts' error, this would panic instead of erroring.
	m := &Manager{cfg: ManagerConfig{}}
	err := m.Reserve(nil, "tenant-1", "run-1", "wf-1", "aws", []*dslpb.MachineGroup{{Count: 1, Cpu: 1}}) //nolint:staticcheck // nil ctx: this path returns before ctx is used.
	if err == nil {
		t.Fatal("expected an error for an unsupported provider")
	}
}

func TestManagerReserveNoDemandIsANoOpBeforeTouchingStore(t *testing.T) {
	// Zero machine groups -> buildQuotaAmounts returns an empty slice ->
	// Reserve must return nil without ever calling m.store (nil here).
	m := &Manager{cfg: ManagerConfig{}}
	err := m.Reserve(nil, "tenant-1", "run-1", "wf-1", "docker", nil) //nolint:staticcheck // nil ctx: this path returns before ctx is used.
	if err != nil {
		t.Fatalf("expected no-op nil error for empty machine groups, got %v", err)
	}
}
