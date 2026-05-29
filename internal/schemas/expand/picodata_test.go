package expand

import "testing"

func TestExpandPicodataCluster(t *testing.T) {
	vms, err := Expand("picodata", map[string]any{"instance_count": float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(vms); got != 3 {
		t.Fatalf("picodata: want 3 db VMs, got %d (%+v)", got, vms)
	}
	if count(vms, RoleDatabase) != 3 {
		t.Fatalf("picodata: want 3 database VMs, got %+v", vms)
	}
}

func TestExpandPicodataDefault(t *testing.T) {
	vms, err := Expand("picodata", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 1 || count(vms, RoleDatabase) != 1 {
		t.Fatalf("picodata default: want 1 database VM, got %+v", vms)
	}
}
