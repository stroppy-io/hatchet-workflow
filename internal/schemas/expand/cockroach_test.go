package expand

import "testing"

func TestExpandCockroachCluster(t *testing.T) {
	vms, err := Expand("cockroach", map[string]any{"topology": "cluster", "node_count": float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	// homogeneous: 3 RoleDatabase nodes, nothing else
	if len(vms) != 3 || count(vms, RoleDatabase) != 3 {
		t.Fatalf("cluster: want 3 database VMs, got %+v", vms)
	}
}

func TestExpandCockroachSingle(t *testing.T) {
	vms, err := Expand("cockroach", map[string]any{"topology": "single"})
	if err != nil {
		t.Fatal(err)
	}
	// default node_count = 1
	if len(vms) != 1 || count(vms, RoleDatabase) != 1 {
		t.Fatalf("single: want 1 database VM, got %+v", vms)
	}
}
