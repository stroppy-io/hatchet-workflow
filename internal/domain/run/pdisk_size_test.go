package run

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

func TestCalculateYDBStoragePdiskGB(t *testing.T) {
	cases := []struct {
		name        string
		script      string
		scaleFactor int
		want        int
	}{
		// User's worked example: 500 TPC-C warehouses → ~50 GB raw → 93 → 186.
		{"tpcc 500 warehouses", "tpcc/tx", 500, 186},
		// Tiny scale still rounds up to a single chunk, then doubles.
		{"tpcc 10 warehouses", "tpcc/tx", 10, 186},
		// Zero / negative scale defaults to 1, same lower bound.
		{"tpcc zero scale", "tpcc/tx", 0, 186},
		// 1500 warehouses → 147 GB raw → 186 → 372.
		{"tpcc 1500 warehouses", "tpcc/tx", 1500, 372},
		// 5000 warehouses → 489 GB raw → 558 → 1116.
		{"tpcc 5000 warehouses", "tpcc/tx", 5000, 1116},
		// TPC-B is 15 MB / unit, much smaller. 1000 units → 15 GB raw → 93 → 186.
		{"tpcb 1000 units", "tpcb/tx", 1000, 186},
		// TPC-B at scale 10000 → 147 GB raw → 186 → 372.
		{"tpcb 10000 units", "tpcb/tx", 10000, 372},
		// Unknown script falls back to the tpcc heuristic.
		{"unknown script", "custom/foo", 500, 186},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CalculateYDBStoragePdiskGB(c.script, c.scaleFactor)
			if got != c.want {
				t.Errorf("CalculateYDBStoragePdiskGB(%q, %d) = %d; want %d", c.script, c.scaleFactor, got, c.want)
			}
			if got%93 != 0 {
				t.Errorf("result %d is not a 93 GiB multiple", got)
			}
		})
	}
}

func TestAdjustYDBStorageDisk_SetsFirstSecondaryDisk(t *testing.T) {
	cfg := types.RunConfig{
		Database: types.DatabaseConfig{
			Kind: types.DatabaseYDB,
			YDB: &types.YDBTopology{
				Storage: types.MachineSpec{
					SecondaryDisks: []types.SecondaryDisk{
						{DeviceName: "ydb-data", SizeGB: 558, Type: "network-ssd-io-m3"},
					},
				},
			},
		},
		Stroppy: types.StroppyConfig{Script: "tpcc/tx", ScaleFactor: 500},
	}
	AdjustYDBStorageDisk(&cfg)
	got := cfg.Database.YDB.Storage.SecondaryDisks[0].SizeGB
	if got != 186 {
		t.Errorf("expected 186 GB for tpcc × 500, got %d", got)
	}
}

func TestAdjustYDBStorageDisk_NoOpWhenNoSecondaryDisks(t *testing.T) {
	cfg := types.RunConfig{
		Database: types.DatabaseConfig{
			Kind: types.DatabaseYDB,
			YDB:  &types.YDBTopology{Storage: types.MachineSpec{}},
		},
		Stroppy: types.StroppyConfig{Script: "tpcc/tx", ScaleFactor: 500},
	}
	AdjustYDBStorageDisk(&cfg)
	if len(cfg.Database.YDB.Storage.SecondaryDisks) != 0 {
		t.Errorf("should not have added secondary disks; got %d", len(cfg.Database.YDB.Storage.SecondaryDisks))
	}
}

func TestAdjustYDBStorageDisk_SkipsNonYDB(t *testing.T) {
	cfg := types.RunConfig{
		Database: types.DatabaseConfig{Kind: types.DatabasePostgres},
		Stroppy:  types.StroppyConfig{Script: "tpcc/tx", ScaleFactor: 500},
	}
	AdjustYDBStorageDisk(&cfg) // shouldn't panic
}
