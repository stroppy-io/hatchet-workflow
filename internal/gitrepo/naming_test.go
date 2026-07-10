package gitrepo

import (
	"strings"
	"testing"
)

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

// TestTenantOrg_FitsGiteaNameLimit is the regression guard for a live 422:
// a tenant id is a 36-char UUID, so the pre-fix name was 52 characters and
// Gitea refused to create the org at all ("[UserName]: MaxSize"). Every org
// repo under it — catalog forks and recipes alike — was therefore
// unreachable.
func TestTenantOrg_FitsGiteaNameLimit(t *testing.T) {
	for _, tenantID := range []string{
		"6efd731f-5f81-4b1d-8ab4-7915341d5eff",
		"00000000-0000-0000-0000-000000000000",
		strings.Repeat("x", 200),
		"Ünicode/../hostile name with spaces",
		"",
	} {
		got := TenantOrg(tenantID)
		if len(got) > maxOrgNameLen {
			t.Errorf("TenantOrg(%q) = %q (%d chars), exceeds Gitea's %d limit",
				tenantID, got, len(got), maxOrgNameLen)
		}
		if !strings.HasPrefix(got, tenantOrgPrefix) {
			t.Errorf("TenantOrg(%q) = %q, lost its prefix", tenantID, got)
		}
	}
}

// TestTenantOrg_TruncationKeepsTenantsDistinct proves the length cap cannot
// collide two tenants whose ids share a long prefix: the hash is keyed on the
// full id, not the truncated body.
func TestTenantOrg_TruncationKeepsTenantsDistinct(t *testing.T) {
	a := TenantOrg("6efd731f-5f81-4b1d-8ab4-7915341d5eff")
	b := TenantOrg("6efd731f-5f81-4b1d-8ab4-7915341d5ef0")
	if a == b {
		t.Fatalf("two distinct tenants collided onto the same org name: %q", a)
	}
}
