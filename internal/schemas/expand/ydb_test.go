package expand

import "testing"

func TestExpandYDBCombined(t *testing.T) {
	db := map[string]any{
		"topology":           "combined",
		"storage_node_count": float64(3),
	}
	vms, err := Expand("ydb", db)
	if err != nil {
		t.Fatal(err)
	}
	// 3 storage nodes, no separate compute nodes in combined mode.
	if got := len(vms); got != 3 {
		t.Fatalf("combined: want 3 db VMs, got %d (%+v)", got, vms)
	}
	if count(vms, RoleDatabase) != 3 || count(vms, RoleReplica) != 0 {
		t.Fatalf("combined: wrong role breakdown: %+v", vms)
	}
}

func TestExpandYDBSplit(t *testing.T) {
	db := map[string]any{
		"topology":            "split",
		"storage_node_count":  float64(3),
		"database_node_count": float64(2),
	}
	vms, err := Expand("ydb", db)
	if err != nil {
		t.Fatal(err)
	}
	// 3 storage + 2 compute = 5
	if got := len(vms); got != 5 {
		t.Fatalf("split: want 5 db VMs, got %d (%+v)", got, vms)
	}
	if count(vms, RoleDatabase) != 3 || count(vms, RoleReplica) != 2 {
		t.Fatalf("split: wrong role breakdown: %+v", vms)
	}
}
