package postgres

import "testing"

// JobCost is the math layer of the quota ledger. Add/Sub correctness keeps
// the per-tenant `used` counter from drifting under concurrent
// claim/finish transactions.

func TestJobCost_AddSumsAllFields(t *testing.T) {
	a := JobCost{CPUs: 4, MemoryMB: 1024, DiskGB: 10, VMCount: 1, RunsRunning: 1}
	b := JobCost{CPUs: 2, MemoryMB: 512, DiskGB: 5, VMCount: 1, RunsRunning: 1}
	got := a.Add(b)
	want := JobCost{CPUs: 6, MemoryMB: 1536, DiskGB: 15, VMCount: 2, RunsRunning: 2}
	if got != want {
		t.Errorf("Add = %+v, want %+v", got, want)
	}
}

func TestJobCost_SubClampsNegative(t *testing.T) {
	// Drift scenario — Finish runs twice for the same job (e.g. reaper +
	// worker race). Second Sub must not push counters negative.
	used := JobCost{CPUs: 0, MemoryMB: 0, DiskGB: 0, VMCount: 0, RunsRunning: 0}
	cost := JobCost{CPUs: 4, MemoryMB: 1024, DiskGB: 10, VMCount: 1, RunsRunning: 1}
	got := used.Sub(cost)
	want := JobCost{}
	if got != want {
		t.Errorf("Sub on zero used = %+v, want zero (no drift)", got)
	}
}

func TestJobCost_SubNormal(t *testing.T) {
	used := JobCost{CPUs: 10, MemoryMB: 4096, DiskGB: 100, VMCount: 5, RunsRunning: 3}
	cost := JobCost{CPUs: 4, MemoryMB: 1024, DiskGB: 10, VMCount: 1, RunsRunning: 1}
	got := used.Sub(cost)
	want := JobCost{CPUs: 6, MemoryMB: 3072, DiskGB: 90, VMCount: 4, RunsRunning: 2}
	if got != want {
		t.Errorf("Sub = %+v, want %+v", got, want)
	}
}
