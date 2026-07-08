package catalog

import (
	"context"
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
