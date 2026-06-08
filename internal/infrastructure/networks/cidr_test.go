package networks

import "testing"

func TestSelectRunCIDRSkipsProviderAndReservedOverlaps(t *testing.T) {
	cidr, err := SelectRunCIDR("run-1", "10.0.0.0/8", []string{
		"10.0.0.0/16",
		"10.1.0.0/16",
		"10.2.0.0/16",
	})
	if err != nil {
		t.Fatalf("select cidr: %v", err)
	}
	for _, blocked := range []string{"10.0.0.0/16", "10.1.0.0/16", "10.2.0.0/16"} {
		if cidr == blocked {
			t.Fatalf("selected blocked cidr %s", cidr)
		}
	}
}

func TestZoneCIDRsSplitsReservedBlock(t *testing.T) {
	got, err := ZoneCIDRs("10.42.0.0/16", []string{"ru-central1-b", "ru-central1-a", "ru-central1-d"})
	if err != nil {
		t.Fatalf("split zones: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("zone cidrs = %v, want 3 entries", got)
	}
	if got["ru-central1-a"] == "" || got["ru-central1-b"] == "" || got["ru-central1-d"] == "" {
		t.Fatalf("zone cidrs missing zones: %v", got)
	}
}

func TestSelectRunCIDRAllocatesRunSubnetsInsideBaseCIDR(t *testing.T) {
	cidr, err := SelectRunCIDR("run-1", "10.1.0.0/16", nil)
	if err != nil {
		t.Fatalf("select cidr: %v", err)
	}
	if got, want := cidr[len(cidr)-3:], "/20"; got != want {
		t.Fatalf("cidr = %q, want /20 child", cidr)
	}
}

func TestSelectRunCIDRRejectsSubnetTooNarrowForYandex(t *testing.T) {
	if _, err := SelectRunCIDR("run-1", "10.0.0.0/21", nil); err == nil {
		t.Fatal("expected /21 to be rejected")
	}
}
