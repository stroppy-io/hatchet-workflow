package netalloc

import "testing"

func TestAssignIPsAvoidsUsed(t *testing.T) {
	out, err := AssignIPs("10.0.0.0/24", []string{"a", "b"}, []string{"10.0.0.5"})
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	// Reserved low addresses skipped (start at .4); .5 is in use -> skipped.
	if out["a"] != "10.0.0.4" {
		t.Errorf("a = %q, want 10.0.0.4", out["a"])
	}
	if out["b"] != "10.0.0.6" {
		t.Errorf("b = %q, want 10.0.0.6 (.5 in use)", out["b"])
	}
}

func TestAssignIPsExhausted(t *testing.T) {
	if _, err := AssignIPs("10.0.0.0/30", []string{"a"}, nil); err == nil {
		t.Fatal("expected exhausted error for a /30 with reserved addresses")
	}
}

func TestAssignIPsEmpty(t *testing.T) {
	out, err := AssignIPs("10.0.0.0/24", nil, nil)
	if err != nil || len(out) != 0 {
		t.Fatalf("want empty map, got %v err=%v", out, err)
	}
}
