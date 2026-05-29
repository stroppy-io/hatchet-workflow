package expand

import "testing"

func count(vms []VM, role Role) int {
	n := 0
	for _, v := range vms {
		if v.Role == role {
			n++
		}
	}
	return n
}

func TestExpandPostgresSingle(t *testing.T) {
	vms, err := Expand("postgres", map[string]any{"topology": "single"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 1 || count(vms, RoleDatabase) != 1 {
		t.Fatalf("single: want 1 database VM, got %+v", vms)
	}
	full := WithWorkload(vms, 1)
	if count(full, RoleWorkload) != 1 || len(full) != 2 {
		t.Fatalf("single+workload: want 2 VMs, got %+v", full)
	}
}

func TestExpandPostgresPatroniHA(t *testing.T) {
	db := map[string]any{
		"topology":      "patroni_ha",
		"replica_count": float64(2),
		"ha":            map[string]any{"dcs": map[string]any{"cluster_size": float64(3)}},
	}
	vms, err := Expand("postgres", db)
	if err != nil {
		t.Fatal(err)
	}
	// 1 primary + 2 replicas + 3 coordinators + 1 proxy = 7
	if got := len(vms); got != 7 {
		t.Fatalf("patroni_ha: want 7 db VMs, got %d (%+v)", got, vms)
	}
	if count(vms, RoleDatabase) != 1 || count(vms, RoleReplica) != 2 ||
		count(vms, RoleCoordinator) != 3 || count(vms, RoleProxy) != 1 {
		t.Fatalf("patroni_ha: wrong role breakdown: %+v", vms)
	}
}

func TestExpandUnknownKind(t *testing.T) {
	if _, err := Expand("oracle", nil); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
