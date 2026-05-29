package expand

import "testing"

func TestExpandMariaDBSingle(t *testing.T) {
	vms, err := Expand("mariadb", map[string]any{"topology": "single"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 1 || count(vms, RoleDatabase) != 1 {
		t.Fatalf("single: want 1 database VM, got %+v", vms)
	}
}

func TestExpandMariaDBPrimaryReplicas(t *testing.T) {
	db := map[string]any{"topology": "primary_replicas", "replica_count": float64(2)}
	vms, err := Expand("mariadb", db)
	if err != nil {
		t.Fatal(err)
	}
	// 1 primary + 2 replicas = 3
	if len(vms) != 3 || count(vms, RoleDatabase) != 1 || count(vms, RoleReplica) != 2 {
		t.Fatalf("primary_replicas: want 1 db + 2 replicas, got %+v", vms)
	}
}

func TestExpandMariaDBGalera(t *testing.T) {
	db := map[string]any{"topology": "galera", "replica_count": float64(2)}
	vms, err := Expand("mariadb", db)
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 3 || count(vms, RoleDatabase) != 1 || count(vms, RoleReplica) != 2 {
		t.Fatalf("galera: want 1 db + 2 replicas, got %+v", vms)
	}
}

func TestExpandMariaDBWithProxy(t *testing.T) {
	db := map[string]any{
		"topology":      "primary_replicas",
		"replica_count": float64(2),
		"proxy":         map[string]any{"use_proxy": true},
	}
	vms, err := Expand("mariadb", db)
	if err != nil {
		t.Fatal(err)
	}
	// 1 primary + 2 replicas + 1 proxy = 4
	if len(vms) != 4 || count(vms, RoleProxy) != 1 {
		t.Fatalf("with proxy: want 4 VMs incl 1 proxy, got %+v", vms)
	}
}
