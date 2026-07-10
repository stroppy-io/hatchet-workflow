package gitrepo

import "testing"

func TestSanitizeSlug_HostileInputContained(t *testing.T) {
	cases := []string{
		"../../etc/passwd",
		"acme/../instance-org",
		"acme;rm -rf /",
		"acme org with spaces",
		"UPPER-Case",
		"日本語スラッグ",
		"",
		"----",
	}
	for _, raw := range cases {
		got := SanitizeSlug(raw)
		if got == "" {
			t.Fatalf("SanitizeSlug(%q) returned empty", raw)
		}
		for _, r := range got {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
			if !ok {
				t.Fatalf("SanitizeSlug(%q) = %q contains disallowed rune %q", raw, got, r)
			}
		}
		if len(got) > maxSanitizedSlugLen {
			t.Fatalf("SanitizeSlug(%q) = %q exceeds max length", raw, got)
		}
	}
}

func TestEntryRepoName_DifferentSlugsNeverCollide(t *testing.T) {
	a := EntryRepoName("provider", "postgres-ha")
	b := EntryRepoName("provider", "postgres_ha")
	if a == b {
		t.Fatalf("distinct raw slugs collided onto the same repo name %q", a)
	}
	// A slug that, sanitized, LOOKS like another kind+slug's mint must still
	// not collide, because the hash suffix is keyed on the raw slug alone.
	c := EntryRepoName("provider", "docker")
	d := EntryRepoName("provider", "DOCKER")
	if c == d {
		t.Fatalf("case-different slugs collided: %q", c)
	}
}

func TestEntryRepoName_Deterministic(t *testing.T) {
	a := EntryRepoName("workflow", "postgres-ha")
	b := EntryRepoName("workflow", "postgres-ha")
	if a != b {
		t.Fatalf("EntryRepoName is not a pure function: %q != %q", a, b)
	}
}

func TestTenantOrg_DistinctFromInstanceOrg(t *testing.T) {
	for _, tid := range []string{"stroppy-instance", "", "abc-123"} {
		got := TenantOrg(tid)
		if got == InstanceOrg {
			t.Fatalf("TenantOrg(%q) = %q collided with InstanceOrg", tid, got)
		}
	}
}

func TestOwnerFor(t *testing.T) {
	if got := OwnerFor(true, ""); got != InstanceOrg {
		t.Fatalf("OwnerFor(instance) = %q, want %q", got, InstanceOrg)
	}
	if got := OwnerFor(false, "tenant-1"); got != TenantOrg("tenant-1") {
		t.Fatalf("OwnerFor(org) = %q, want %q", got, TenantOrg("tenant-1"))
	}
}
