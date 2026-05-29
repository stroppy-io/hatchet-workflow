package expand

import "testing"

func TestExpandYDBManaged(t *testing.T) {
	vms, err := Expand("ydb-managed", map[string]any{"type": "serverless"})
	if err != nil {
		t.Fatal(err)
	}
	// Managed YDB provisions no self-hosted database machines.
	if len(vms) != 0 {
		t.Fatalf("ydb-managed: want 0 db VMs, got %+v", vms)
	}

	full := WithWorkload(vms, 1)
	if count(full, RoleWorkload) != 1 || len(full) != 1 {
		t.Fatalf("ydb-managed+workload: want exactly 1 workload VM, got %+v", full)
	}
}
