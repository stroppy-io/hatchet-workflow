package catalog

import (
	"context"
	"errors"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func TestSeedOrgCatalog_LinksEveryInstanceEntry(t *testing.T) {
	repo := newFakeEntryRepo()
	// two pre-existing instance entries
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1))
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_WORKFLOW, "tpcc", 1))

	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("SeedOrgCatalog: %v", err)
	}

	orgEntries, err := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if err != nil || len(orgEntries) != 1 {
		t.Fatalf("expected 1 linked provider, got %d (err=%v)", len(orgEntries), err)
	}
	if orgEntries[0].GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("origin = %v, want LINKED", orgEntries[0].GetOrigin())
	}
	if orgEntries[0].GetSourceRef() != "" {
		t.Fatal("LINKED row must not have its own source_ref before first fork")
	}
}

func TestSeedOrgCatalog_Idempotent(t *testing.T) {
	repo := newFakeEntryRepo()
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1))
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})

	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("1st SeedOrgCatalog: %v", err)
	}
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("2nd SeedOrgCatalog: %v", err)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(orgEntries) != 1 {
		t.Fatalf("expected exactly 1 linked entry after 2 seed calls, got %d", len(orgEntries))
	}
}

func TestSeedOrgCatalog_SeedsBuiltinDockerFirst(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := NewService(Deps{
		Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{},
		BuiltinProviders: map[string]map[string][]byte{"docker": {"manifest.yaml": []byte("name: docker\nprovides:\n  - machines\n")}},
	})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("SeedOrgCatalog: %v", err)
	}
	instanceEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_INSTANCE, "", catalogpb.Kind_KIND_PROVIDER)
	if len(instanceEntries) != 1 || instanceEntries[0].GetSlug() != "docker" {
		t.Fatalf("expected docker seeded as instance entry, got %+v", instanceEntries)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(orgEntries) != 1 || orgEntries[0].GetSlug() != "docker" {
		t.Fatalf("expected docker linked into org catalog, got %+v", orgEntries)
	}
}

func TestSeedOrgCatalog_BuiltinSeededOnceAcrossTenants(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := NewService(Deps{
		Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{},
		BuiltinProviders: map[string]map[string][]byte{"docker": {"manifest.yaml": []byte("name: docker\nprovides:\n  - machines\n")}},
	})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("seed tenant-1: %v", err)
	}
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-2"); err != nil {
		t.Fatalf("seed tenant-2: %v", err)
	}
	instanceEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_INSTANCE, "", catalogpb.Kind_KIND_PROVIDER)
	if len(instanceEntries) != 1 {
		t.Fatalf("expected docker seeded exactly once as an instance entry across tenants, got %d", len(instanceEntries))
	}
	for _, tenantID := range []string{"tenant-1", "tenant-2"} {
		orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, tenantID, catalogpb.Kind_KIND_PROVIDER)
		if len(orgEntries) != 1 || orgEntries[0].GetSlug() != "docker" {
			t.Fatalf("expected docker linked into %s's org catalog, got %+v", tenantID, orgEntries)
		}
	}
}

// TestBackfillOrgCatalogs_LinksPreexistingTenant covers the reported bug: a
// tenant that predates the catalog (never went through iamService's
// SeedOrgCatalog-on-create path) gets LINKED rows for every LEVEL_INSTANCE
// entry, both KIND_PROVIDER and KIND_WORKFLOW, once BackfillOrgCatalogs runs.
func TestBackfillOrgCatalogs_LinksPreexistingTenant(t *testing.T) {
	repo := newFakeEntryRepo()
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "docker", 1))
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_WORKFLOW, "postgres-ha", 1))
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})

	svc.BackfillOrgCatalogs(context.Background(), []string{"preexisting-tenant"}, func(tenantID string, err error) {
		t.Fatalf("unexpected backfill error for %s: %v", tenantID, err)
	})

	providers, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "preexisting-tenant", catalogpb.Kind_KIND_PROVIDER)
	workflows, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "preexisting-tenant", catalogpb.Kind_KIND_WORKFLOW)
	if len(providers) != 1 || providers[0].GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("expected 1 linked provider for preexisting tenant, got %+v", providers)
	}
	if len(workflows) != 1 || workflows[0].GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("expected 1 linked workflow for preexisting tenant, got %+v", workflows)
	}
}

// TestBackfillOrgCatalogs_Idempotent covers "running the backfill twice adds
// nothing" — every boot calls BackfillOrgCatalogs unconditionally, so it must
// stay a no-op once a tenant is fully linked.
func TestBackfillOrgCatalogs_Idempotent(t *testing.T) {
	repo := newFakeEntryRepo()
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "docker", 1))
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})

	noErr := func(tenantID string, err error) { t.Fatalf("unexpected backfill error for %s: %v", tenantID, err) }
	svc.BackfillOrgCatalogs(context.Background(), []string{"tenant-1"}, noErr)
	svc.BackfillOrgCatalogs(context.Background(), []string{"tenant-1"}, noErr)

	providers, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(providers) != 1 {
		t.Fatalf("expected exactly 1 linked provider after 2 backfill runs, got %d", len(providers))
	}
}

// TestBackfillOrgCatalogs_OnlyMissingLinks covers a tenant that already has
// some (but not all) links — e.g. one created after the catalog existed but
// before a second builtin provider was added: backfill must add only the
// missing link, not duplicate the existing one.
func TestBackfillOrgCatalogs_OnlyMissingLinks(t *testing.T) {
	repo := newFakeEntryRepo()
	dockerInstance := repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "docker", 1))
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1))
	// tenant-1 already has docker linked (as if seeded on tenant creation) —
	// built from orgEntry and then adjusted to LINKED/sourced, matching what
	// SeedOrgCatalog itself would have written.
	dockerLink := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "docker", 1)
	dockerLink.Origin = catalogpb.Origin_ORIGIN_LINKED
	dockerLink.SourceEntryId = dockerInstance.GetEntity().GetId()
	repo.mustCreate(t, dockerLink)

	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})
	svc.BackfillOrgCatalogs(context.Background(), []string{"tenant-1"}, func(tenantID string, err error) {
		t.Fatalf("unexpected backfill error for %s: %v", tenantID, err)
	})

	providers, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(providers) != 2 {
		t.Fatalf("expected docker (pre-existing) + yandex (backfilled) = 2 linked providers, got %d: %+v", len(providers), providers)
	}
	var dockerCount int
	for _, p := range providers {
		if p.GetSlug() == "docker" {
			dockerCount++
		}
	}
	if dockerCount != 1 {
		t.Fatalf("expected docker's pre-existing link to stay singular (not duplicated), got %d", dockerCount)
	}
}

// failingCreateRepo wraps fakeEntryRepo so Create fails for one tenant's
// LEVEL_ORG rows, simulating a per-tenant backfill failure (e.g. a
// tenant-scoped store outage) without affecting any other tenant.
type failingCreateRepo struct {
	*fakeEntryRepo
	failTenantID string
}

func (r *failingCreateRepo) Create(ctx context.Context, e *catalogpb.CatalogEntry) error {
	if e.GetLevel() == catalogpb.Level_LEVEL_ORG && e.GetEntity().GetTenantId() == r.failTenantID {
		return errors.New("simulated create failure")
	}
	return r.fakeEntryRepo.Create(ctx, e)
}

// TestBackfillOrgCatalogs_FailingTenantDoesNotAbortOthers covers the failure
// policy from the task: a per-tenant error is reported via onError, not
// returned/propagated, so one bad tenant cannot block backfill for the rest
// (mirrors 1c8ad7f5's "log and continue" posture for the gitea bootstrap).
func TestBackfillOrgCatalogs_FailingTenantDoesNotAbortOthers(t *testing.T) {
	repo := &failingCreateRepo{fakeEntryRepo: newFakeEntryRepo(), failTenantID: "bad-tenant"}
	repo.mustCreate(t, instanceEntry(catalogpb.Kind_KIND_PROVIDER, "docker", 1))
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})

	var failed []string
	svc.BackfillOrgCatalogs(context.Background(), []string{"bad-tenant", "good-tenant"}, func(tenantID string, err error) {
		failed = append(failed, tenantID)
	})

	if len(failed) != 1 || failed[0] != "bad-tenant" {
		t.Fatalf("onError calls = %v, want exactly [bad-tenant]", failed)
	}
	goodProviders, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "good-tenant", catalogpb.Kind_KIND_PROVIDER)
	if len(goodProviders) != 1 {
		t.Fatalf("expected good-tenant to still be linked despite bad-tenant's failure, got %d", len(goodProviders))
	}
	badProviders, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "bad-tenant", catalogpb.Kind_KIND_PROVIDER)
	if len(badProviders) != 0 {
		t.Fatalf("expected bad-tenant to have no linked rows after a failed create, got %d", len(badProviders))
	}
}
