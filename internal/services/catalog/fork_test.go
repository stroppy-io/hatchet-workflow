package catalog

import (
	"context"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func TestForkEntry_LinkedRowForks(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instanceRef, _ := store.Write(context.Background(), BundleIdentity{}, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	linked := orgEntries[0]

	forked, err := svc.ForkEntry(context.Background(), "tenant-1", linked.GetEntity().GetId())
	if err != nil {
		t.Fatalf("ForkEntry: %v", err)
	}
	if forked.GetOrigin() != catalogpb.Origin_ORIGIN_FORKED {
		t.Fatalf("origin = %v, want FORKED", forked.GetOrigin())
	}
	if forked.GetVersion() != linked.GetVersion()+1 {
		t.Fatalf("version = %d, want %d", forked.GetVersion(), linked.GetVersion()+1)
	}
	if forked.GetSourceRef() == "" || forked.GetSourceRef() == instanceRef {
		t.Fatal("forked row must have its own distinct source_ref")
	}
	if forked.GetSourceEntryId() != instance.GetEntity().GetId() {
		t.Fatal("forked row must keep source_entry_id for lineage")
	}

	// instance row is untouched
	stillInstance, err := repo.Get(context.Background(), catalogpb.Level_LEVEL_INSTANCE, "", instance.GetEntity().GetId())
	if err != nil || stillInstance.GetOrigin() != catalogpb.Origin_ORIGIN_NATIVE || stillInstance.GetSourceRef() != instanceRef {
		t.Fatal("instance row must not be mutated by ForkEntry")
	}
}

// TestForkEntry_SkipsExistingVersion locks the version-collision fix:
// stamping entry.GetVersion()+1 unconditionally is not safe when that
// version is already occupied at this (level, tenant, kind, slug) scope
// (e.g. a previous fork or an org-native row already claimed it) — it would
// violate uq_catalog_entries_scope_slug_version. ForkEntry must instead pick
// the actual next-free version for the scope.
func TestForkEntry_SkipsExistingVersion(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instanceRef, _ := store.Write(context.Background(), BundleIdentity{}, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	if err := svc.SeedOrgCatalog(context.Background(), "tenant-1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	linked := orgEntries[0]
	if linked.GetVersion() != 1 {
		t.Fatalf("precondition: linked version = %d, want 1", linked.GetVersion())
	}

	// Simulate version 2 already occupied at this scope (e.g. by an earlier
	// fork/update this test doesn't otherwise model) — entry.GetVersion()+1
	// would collide with this row.
	occupied := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 2)
	repo.mustCreate(t, occupied)

	forked, err := svc.ForkEntry(context.Background(), "tenant-1", linked.GetEntity().GetId())
	if err != nil {
		t.Fatalf("ForkEntry: %v", err)
	}
	if forked.GetVersion() != 3 {
		t.Fatalf("version = %d, want 3 (next free version, skipping occupied v2)", forked.GetVersion())
	}
}

func TestForkEntry_OtherTenantsLinkedRowUntouched(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instanceRef, _ := store.Write(context.Background(), BundleIdentity{}, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	svc.SeedOrgCatalog(context.Background(), "tenant-A")
	svc.SeedOrgCatalog(context.Background(), "tenant-B")

	aEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-A", catalogpb.Kind_KIND_PROVIDER)
	svc.ForkEntry(context.Background(), "tenant-A", aEntries[0].GetEntity().GetId())

	bEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-B", catalogpb.Kind_KIND_PROVIDER)
	if bEntries[0].GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("tenant-B's row must remain LINKED, got %v", bEntries[0].GetOrigin())
	}
}

func TestForkEntry_NativeRowIsNoOp(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})

	ref, _ := store.Write(context.Background(), BundleIdentity{}, "", map[string][]byte{"manifest.yaml": []byte("name: native\n")})
	native := &catalogpb.CatalogEntry{
		Entity:    instanceEntry(catalogpb.Kind_KIND_PROVIDER, "native", 1).GetEntity(),
		Level:     catalogpb.Level_LEVEL_ORG,
		Kind:      catalogpb.Kind_KIND_PROVIDER,
		Slug:      "native",
		Version:   1,
		Origin:    catalogpb.Origin_ORIGIN_NATIVE,
		SourceRef: ref,
	}
	native.Entity.TenantId = "tenant-1"
	repo.mustCreate(t, native)

	got, err := svc.ForkEntry(context.Background(), "tenant-1", native.GetEntity().GetId())
	if err != nil {
		t.Fatalf("ForkEntry: %v", err)
	}
	if got.GetOrigin() != catalogpb.Origin_ORIGIN_NATIVE || got.GetEntity().GetId() != native.GetEntity().GetId() {
		t.Fatal("ForkEntry on a NATIVE row must be a no-op returning the same row")
	}
	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(orgEntries) != 1 {
		t.Fatalf("len(orgEntries) = %d, want 1 (no fork row created)", len(orgEntries))
	}
}

func TestUpdateOrgProvider_ForksLinkedRow(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instanceRef, _ := store.Write(context.Background(), BundleIdentity{}, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	svc.SeedOrgCatalog(context.Background(), "tenant-1")
	linked, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)

	resp, err := svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: linked[0].GetEntity().GetId(),
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("UpdateOrgProvider: %v", err)
	}
	if resp.GetEntry().GetOrigin() != catalogpb.Origin_ORIGIN_FORKED {
		t.Fatalf("origin = %v, want FORKED", resp.GetEntry().GetOrigin())
	}
	if resp.GetEntry().GetEntity().GetId() == linked[0].GetEntity().GetId() {
		t.Fatal("fork-on-edit must produce a new row id, not mutate the LINKED row in place")
	}

	// the original LINKED row is untouched.
	stillLinked, err := repo.Get(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", linked[0].GetEntity().GetId())
	if err != nil || stillLinked.GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatal("original LINKED row must not be mutated by UpdateOrgProvider")
	}
}

// TestUpdateOrgProvider_EditingForkedRowCreatesAnotherNewVersion locks the
// owner-mandated immutability invariant for a second edit against an
// already-FORKED row. Before this task, a second UpdateOrgProvider call
// against a FORKED row mutated it in place, keeping its id/version — this
// test used to assert exactly that
// (TestUpdateOrgProvider_ForkedRowUpdatesInPlaceWithoutReForking). It is
// rewritten here to assert the opposite: it must create yet another new
// FORKED version, and the first FORKED version's bytes/summary must be
// provably unchanged.
func TestUpdateOrgProvider_EditingForkedRowCreatesAnotherNewVersion(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	instanceRef, _ := store.Write(context.Background(), BundleIdentity{}, "", map[string][]byte{"manifest.yaml": []byte("name: yandex\n")})
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = instanceRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{Entries: repo, Bundles: store, Check: stubChecker(nil), Authn: fakeAuthn{}})
	svc.SeedOrgCatalog(context.Background(), "tenant-1")
	linked, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)

	first, err := svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: linked[0].GetEntity().GetId(),
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	firstID := first.GetEntry().GetEntity().GetId()
	firstVersion := first.GetEntry().GetVersion()
	firstSourceRef := first.GetEntry().GetSourceRef()

	second, err := svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: firstID,
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n  - volumes\n")},
	})
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if second.GetEntry().GetEntity().GetId() == firstID {
		t.Fatal("editing a FORKED row must create a new row/version, not mutate it in place")
	}
	if second.GetEntry().GetVersion() != firstVersion+1 {
		t.Fatalf("version = %d, want %d (next free version)", second.GetEntry().GetVersion(), firstVersion+1)
	}
	if second.GetEntry().GetOrigin() != catalogpb.Origin_ORIGIN_FORKED {
		t.Fatalf("origin = %v, want FORKED (origin carried forward from the row being edited)", second.GetEntry().GetOrigin())
	}

	// The first FORKED version must be provably untouched by the second edit.
	stillFirst, err := repo.Get(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", firstID)
	if err != nil {
		t.Fatalf("get first forked version: %v", err)
	}
	if stillFirst.GetVersion() != firstVersion || stillFirst.GetSourceRef() != firstSourceRef {
		t.Fatal("first forked version's version/source_ref must not change after a later edit")
	}
	if len(stillFirst.GetSummary().GetProvides()) != 1 {
		t.Fatalf("first forked version's summary changed: provides = %v, want 1 entry ([machines])", stillFirst.GetSummary().GetProvides())
	}

	orgEntries, _ := repo.List(context.Background(), catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if len(orgEntries) != 3 {
		t.Fatalf("len(orgEntries) = %d, want 3 (original LINKED row + two FORKED versions)", len(orgEntries))
	}
}
