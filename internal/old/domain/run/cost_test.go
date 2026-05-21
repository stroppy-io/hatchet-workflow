package run

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// EstimateRunCost is a pure helper, so each case constructs the smallest
// RunConfig that exercises one accounting path. Assertions check both raw
// totals and the "RunsRunning=1 always" invariant the scheduler depends on.

func TestEstimateRunCost_PostgresHATopology(t *testing.T) {
	cfg := types.RunConfig{
		Provider: types.ProviderYandex,
		Database: types.DatabaseConfig{
			Kind: types.DatabasePostgres,
			Postgres: &types.PostgresTopology{
				Master:   types.MachineSpec{Role: types.RoleDatabase, Count: 1, CPUs: 4, MemoryMB: 8192, DiskGB: 100},
				Replicas: []types.MachineSpec{{Role: types.RoleDatabase, Count: 2, CPUs: 4, MemoryMB: 8192, DiskGB: 100}},
				HAProxy:  &types.MachineSpec{Role: types.RoleProxy, Count: 1, CPUs: 2, MemoryMB: 2048, DiskGB: 20},
			},
		},
		Stroppy: types.StroppyConfig{
			Machine: &types.MachineSpec{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
		},
	}
	c := EstimateRunCost(cfg)
	// 3 DB nodes (1 master + 2 replicas, all 4c/8GB/100GB) + 1 HAProxy (2c/2GB/20GB) + 1 stroppy (2c/4GB/50GB).
	wantCPU := 3*4 + 1*2 + 1*2
	wantMem := 3*8192 + 1*2048 + 1*4096
	wantDisk := 3*100 + 1*20 + 1*50
	wantVMs := 3 + 1 + 1
	if c.CPUs != wantCPU {
		t.Errorf("CPUs = %d, want %d", c.CPUs, wantCPU)
	}
	if c.MemoryMB != wantMem {
		t.Errorf("MemoryMB = %d, want %d", c.MemoryMB, wantMem)
	}
	if c.DiskGB != wantDisk {
		t.Errorf("DiskGB = %d, want %d", c.DiskGB, wantDisk)
	}
	if c.VMCount != wantVMs {
		t.Errorf("VMCount = %d, want %d", c.VMCount, wantVMs)
	}
	if c.RunsRunning != 1 {
		t.Errorf("RunsRunning = %d, want 1", c.RunsRunning)
	}
}

func TestEstimateRunCost_ExternalDBOnlyStroppy(t *testing.T) {
	// BYO database — the only machine we provision is the stroppy runner.
	cfg := types.RunConfig{
		Provider:   types.ProviderYandex,
		ExternalDB: &types.ExternalDBConfig{Endpoint: "10.0.0.1:5432"},
		Database:   types.DatabaseConfig{Kind: types.DatabasePostgres, Version: "16"},
		Stroppy: types.StroppyConfig{
			Machine: &types.MachineSpec{Role: types.RoleStroppy, Count: 1, CPUs: 8, MemoryMB: 16384, DiskGB: 80},
		},
	}
	c := EstimateRunCost(cfg)
	if c.VMCount != 1 {
		t.Errorf("VMCount = %d, want 1 (stroppy runner only)", c.VMCount)
	}
	if c.CPUs != 8 || c.MemoryMB != 16384 || c.DiskGB != 80 {
		t.Errorf("got %+v, want stroppy-only", c)
	}
}

func TestEstimateRunCost_DockerCountsStroppyRunner(t *testing.T) {
	// FillMachinesFromTopology unconditionally appends a stroppy runner —
	// for docker that's a container co-located on the host but it still
	// counts toward the cost ledger so quotas treat docker and yandex
	// uniformly.
	cfg := types.RunConfig{
		Provider: types.ProviderDocker,
		Database: types.DatabaseConfig{
			Kind: types.DatabaseMySQL,
			MySQL: &types.MySQLTopology{
				Primary: types.MachineSpec{Role: types.RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
			},
		},
		Stroppy: types.StroppyConfig{Machine: &types.MachineSpec{CPUs: 4, MemoryMB: 8192, DiskGB: 80}},
	}
	c := EstimateRunCost(cfg)
	if c.VMCount != 2 {
		t.Errorf("VMCount = %d, want 2 (db + stroppy)", c.VMCount)
	}
	if c.CPUs != 2+4 {
		t.Errorf("CPUs = %d, want 6", c.CPUs)
	}
}

func TestEstimateRunCost_SecondaryDisksCounted(t *testing.T) {
	cfg := types.RunConfig{
		Provider: types.ProviderYandex,
		Database: types.DatabaseConfig{
			Kind: types.DatabaseYDB,
			YDB: &types.YDBTopology{
				Storage: types.MachineSpec{
					Role: types.RoleYDBStorage, Count: 3, CPUs: 4, MemoryMB: 16384, DiskGB: 50,
					SecondaryDisks: []types.SecondaryDisk{{DeviceName: "data", SizeGB: 372}},
				},
			},
		},
	}
	c := EstimateRunCost(cfg)
	// 3 storage nodes × (50 boot + 372 data) = 1266 GB,
	// plus FillMachinesFromTopology's default stroppy runner (20 GB).
	wantDisk := 3*(50+372) + 20
	if c.DiskGB != wantDisk {
		t.Errorf("DiskGB = %d, want %d (3 storage + stroppy default)", c.DiskGB, wantDisk)
	}
	if c.VMCount != 4 {
		t.Errorf("VMCount = %d, want 4 (3 storage + stroppy)", c.VMCount)
	}
}
