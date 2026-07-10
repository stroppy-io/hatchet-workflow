package catalog

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/gitrepo"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

/*
	===== healSourceRef: the legacy-store -> GitEntryBundleStore migration
	seam =====

	These tests exercise a catalog_entries row whose source_ref was minted by
	a superseded store (FSBundleStore's bare sha256 hex, or the retired
	single-monorepo GitBundleStore's 4-segment ref) and becomes unreadable
	once the active BundleStore is the repo-per-entry GitEntryBundleStore,
	which hard-rejects any ref that is not the commit-pinned
	"git:<owner>/<repo>@<sha>" shape. healSourceRef (catalog.go) is what
	repairs this lazily, on first read, without a boot-time migration pass —
	materializing the entry's own per-item repo for the first time in the
	process. See healSourceRef's doc comment for the full rationale.
*/

// TestHealSourceRef_LegacyBareRefMigratesIntoGitStore proves the core claim:
// reading a bare-sha (legacy FSBundleStore) source_ref while a
// GitEntryBundleStore is the active store (a) succeeds, (b) returns the
// original bundle bytes, (c) actually lands the bundle's files in the
// entry's own fake Gitea repo's tree — not merely rewrites a DB pointer —
// and (d) persists the new commit-pinned ref onto the owning row so a
// second read never touches the legacy store again.
func TestHealSourceRef_LegacyBareRefMigratesIntoGitStore(t *testing.T) {
	ctx := context.Background()

	legacy, err := NewFSBundleStore(t.TempDir())
	if err != nil {
		t.Fatalf("new fs store: %v", err)
	}
	files := map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")}
	legacyRef, err := legacy.Write(ctx, BundleIdentity{}, "", files)
	if err != nil {
		t.Fatalf("legacy write: %v", err)
	}
	if IsGitCommitRef(legacyRef) {
		t.Fatalf("legacy ref %q unexpectedly commit-ref-shaped", legacyRef)
	}

	cli := newFakeGitClient()
	active := NewGitEntryBundleStore(cli)

	repo := newFakeEntryRepo()
	owner := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	owner.SourceRef = legacyRef
	repo.mustCreate(t, owner)

	healedRef, err := healSourceRef(ctx, repo, active, legacy, owner, legacyRef)
	if err != nil {
		t.Fatalf("healSourceRef: %v", err)
	}
	if !IsGitCommitRef(healedRef) {
		t.Fatalf("healed ref %q is not commit-ref-shaped", healedRef)
	}

	// (b) content is readable, byte-identical, through the ACTIVE store.
	got, err := active.Read(ctx, healedRef)
	if err != nil {
		t.Fatalf("read healed ref via active store: %v", err)
	}
	if string(got["manifest.yaml"]) != string(files["manifest.yaml"]) {
		t.Fatalf("healed content = %q, want %q", got["manifest.yaml"], files["manifest.yaml"])
	}

	// (c) the bundle's files are actually committed into the entry's own
	// fake git repo's tree — decode the healed ref to find it, bypassing the
	// BundleStore abstraction, to prove the content really moved.
	healedOwner, healedRepo, healedSha, err := DecodeGitCommitRef(healedRef)
	if err != nil {
		t.Fatalf("decode healed ref: %v", err)
	}
	paths, err := cli.ListTree(ctx, healedOwner, healedRepo, healedSha)
	if err != nil {
		t.Fatalf("list tree: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected the healed bundle's files to be committed into the git repo's tree, found none")
	}

	// (d) the owning row's source_ref was rewritten in the entry repo.
	persisted, err := repo.Get(ctx, catalogpb.Level_LEVEL_INSTANCE, "", owner.GetEntity().GetId())
	if err != nil {
		t.Fatalf("get persisted owner: %v", err)
	}
	if persisted.GetSourceRef() != healedRef {
		t.Fatalf("persisted source_ref = %q, want healed ref %q", persisted.GetSourceRef(), healedRef)
	}

	// Second heal of the now-commit-ref-shaped ref is a pure pass-through:
	// no second legacy read, no second git commit.
	repoState := cli.repos[cli.key(healedOwner, healedRepo)]
	commitsBefore := repoState.commitN
	again, err := healSourceRef(ctx, repo, active, legacy, persisted, persisted.GetSourceRef())
	if err != nil {
		t.Fatalf("second heal: %v", err)
	}
	if again != healedRef {
		t.Fatalf("second heal changed the ref: %q vs %q", again, healedRef)
	}
	if repoState.commitN != commitsBefore {
		t.Fatalf("second heal committed again: %d vs %d", repoState.commitN, commitsBefore)
	}
}

// erroringBundleStore fails any call — used to prove a commit-ref-shaped ref
// never touches the legacy store at all (a healed/native ref is a pure
// pass-through).
type erroringBundleStore struct{}

func (erroringBundleStore) Write(context.Context, BundleIdentity, string, map[string][]byte) (string, error) {
	panic("erroringBundleStore.Write should never be called")
}

func (erroringBundleStore) Read(context.Context, string) (map[string][]byte, error) {
	panic("erroringBundleStore.Read should never be called")
}

func (erroringBundleStore) Fork(context.Context, BundleIdentity, string) (string, error) {
	panic("erroringBundleStore.Fork should never be called")
}

var _ BundleStore = erroringBundleStore{}

// TestHealSourceRef_GitRefIsPurelyPassedThrough proves dispatch on scheme: a
// ref already shaped "git:owner/repo@sha" is returned unchanged, and the
// legacy store is never consulted (it would panic if it were).
func TestHealSourceRef_GitRefIsPurelyPassedThrough(t *testing.T) {
	cli := newFakeGitClient()
	active := NewGitEntryBundleStore(cli)
	files := map[string][]byte{"manifest.yaml": []byte("name: docker\n")}
	ref, err := active.Write(context.Background(), instanceProvider("docker"), "", files)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	repo := newFakeEntryRepo()
	owner := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "docker", 1)
	owner.SourceRef = ref
	repo.mustCreate(t, owner)

	got, err := healSourceRef(context.Background(), repo, active, erroringBundleStore{}, owner, ref)
	if err != nil {
		t.Fatalf("healSourceRef: %v", err)
	}
	if got != ref {
		t.Fatalf("healSourceRef changed an already-git ref: %q vs %q", got, ref)
	}
}

// TestHealSourceRef_NoLegacyConfiguredIsNoOp proves a nil LegacyBundles (the
// default, never-swapped-backend case — see Deps.LegacyBundles's doc) leaves
// a bare ref untouched rather than erroring: this is the behavior every
// existing deployment that has never swapped BundleStore backends must keep.
func TestHealSourceRef_NoLegacyConfiguredIsNoOp(t *testing.T) {
	repo := newFakeEntryRepo()
	owner := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	owner.SourceRef = "deadbeef"
	repo.mustCreate(t, owner)

	got, err := healSourceRef(context.Background(), repo, NewMemoryBundleStore(), nil, owner, "deadbeef")
	if err != nil {
		t.Fatalf("healSourceRef: %v", err)
	}
	if got != "deadbeef" {
		t.Fatalf("healSourceRef mutated a bare ref with no legacy store configured: %q", got)
	}
}

// TestHealSourceRef_LegacyMonorepoRefMigrates proves the SECOND legacy shape
// — a ref minted by the retired single-repo GitBundleStore
// ("git:owner/repo/branch/hash") — also heals into the active
// GitEntryBundleStore, via GitMonorepoBundleStore as the legacy reader (the
// composite dualLegacyBundleStore internal/app/run.go wires dispatches to
// this reader for exactly this ref shape).
func TestHealSourceRef_LegacyMonorepoRefMigrates(t *testing.T) {
	ctx := context.Background()
	cli := newFakeGitClient()

	// Seed the legacy monorepo layout via the (unused-in-production, but
	// interface-complete) GitMonorepoBundleStore.Write.
	legacyMonorepo := NewGitMonorepoBundleStore(cli)
	if err := cli.EnsureRepo(ctx, "", "instance-catalog", true); err != nil {
		t.Fatalf("ensure legacy repo: %v", err)
	}
	files := map[string][]byte{"manifest.yaml": []byte("name: yandex\n")}
	legacyRef, err := legacyMonorepo.Write(ctx, BundleIdentity{}, "", files)
	if err != nil {
		t.Fatalf("legacy monorepo write: %v", err)
	}
	if !IsLegacyMonorepoRef(legacyRef) {
		t.Fatalf("seed ref %q is not legacy-monorepo-shaped", legacyRef)
	}

	active := NewGitEntryBundleStore(cli)
	repo := newFakeEntryRepo()
	owner := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	owner.SourceRef = legacyRef
	repo.mustCreate(t, owner)

	healedRef, err := healSourceRef(ctx, repo, active, legacyMonorepo, owner, legacyRef)
	if err != nil {
		t.Fatalf("healSourceRef: %v", err)
	}
	if !IsGitCommitRef(healedRef) {
		t.Fatalf("healed ref %q is not commit-ref-shaped", healedRef)
	}
	got, err := active.Read(ctx, healedRef)
	if err != nil {
		t.Fatalf("read healed ref: %v", err)
	}
	if string(got["manifest.yaml"]) != string(files["manifest.yaml"]) {
		t.Fatalf("healed content = %q, want %q", got["manifest.yaml"], files["manifest.yaml"])
	}
}

/*
	===== end-to-end: a previously FS-seeded entry becomes readable through
	the git-backed path via the Service/CatalogProviderResolver layers =====
*/

// TestGetOrgProviderFiles_HealsLegacySeedOnRead is the end-to-end
// reproduction: a LEVEL_INSTANCE row seeded while FSBundleStore was active
// (source_ref = bare sha256 hex, exactly what ensureBuiltinKind/seed.go
// writes), linked into an org's catalog by SeedOrgCatalog, then read via
// GetOrgProviderFiles AFTER the service has been reconfigured with a
// GitEntryBundleStore as the active store and the old FSBundleStore wired in
// as LegacyBundles (mirroring run.go's wiring when GITEA_TOKEN is set on a
// stand that previously ran without it). The call succeeds, returns the
// original bytes, AND the instance row's source_ref is rewritten to a
// commit-pinned ref backed by content actually committed in the entry's own
// fake git repo.
func TestGetOrgProviderFiles_HealsLegacySeedOnRead(t *testing.T) {
	ctx := context.Background()

	legacy, err := NewFSBundleStore(t.TempDir())
	if err != nil {
		t.Fatalf("new fs store: %v", err)
	}
	seedFiles := map[string][]byte{"manifest.yaml": []byte("name: docker\nprovides:\n  - machines\n")}
	legacyRef, err := legacy.Write(ctx, BundleIdentity{}, "", seedFiles)
	if err != nil {
		t.Fatalf("legacy write: %v", err)
	}

	repo := newFakeEntryRepo()
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "docker", 1)
	instance.SourceRef = legacyRef
	repo.mustCreate(t, instance)

	cli := newFakeGitClient()
	active := NewGitEntryBundleStore(cli)

	svc := NewService(Deps{
		Entries:       repo,
		Bundles:       active,
		LegacyBundles: legacy,
		Check:         stubChecker(nil),
		Authn:         fakeAuthn{},
	})
	if err := svc.SeedOrgCatalog(ctx, "tenant-1"); err != nil {
		t.Fatalf("SeedOrgCatalog: %v", err)
	}
	orgEntries, err := repo.List(ctx, catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	if err != nil || len(orgEntries) != 1 {
		t.Fatalf("org entries = %v, err %v; want exactly 1", orgEntries, err)
	}
	linked := orgEntries[0]
	if linked.GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("origin = %v, want LINKED", linked.GetOrigin())
	}
	if linked.GetSourceRef() != "" {
		t.Fatal("SeedOrgCatalog-created LINKED row must carry no source_ref of its own")
	}

	resp, err := svc.GetOrgProviderFiles(ctx, &catalogpb.GetOrgProviderFilesRequest{
		TenantId: "tenant-1", Id: linked.GetEntity().GetId(),
	})
	if err != nil {
		t.Fatalf("GetOrgProviderFiles: %v", err)
	}
	if string(resp.GetFiles()["manifest.yaml"]) != string(seedFiles["manifest.yaml"]) {
		t.Fatalf("files = %v, want %v", resp.GetFiles(), seedFiles)
	}

	// The INSTANCE row (not the LINKED row, which never owned the ref) must
	// now carry a commit-pinned source_ref, backed by content actually
	// present in its own git repo.
	healedInstance, err := repo.Get(ctx, catalogpb.Level_LEVEL_INSTANCE, "", instance.GetEntity().GetId())
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if !IsGitCommitRef(healedInstance.GetSourceRef()) {
		t.Fatalf("instance source_ref = %q, want a commit-pinned ref after healing", healedInstance.GetSourceRef())
	}
	hOwner, hRepo, hSha, err := DecodeGitCommitRef(healedInstance.GetSourceRef())
	if err != nil {
		t.Fatalf("decode healed ref: %v", err)
	}
	paths, err := cli.ListTree(ctx, hOwner, hRepo, hSha)
	if err != nil {
		t.Fatalf("list tree: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected the migrated bundle to be committed into the actual git repo, found none")
	}
}

// TestCatalogProviderResolver_ResolveProvider_HealsLegacyRef proves the DSL
// compile-time provider resolution path (a distinct read call site from
// GetOrgProviderFiles) also heals a legacy ref rather than failing outright.
func TestCatalogProviderResolver_ResolveProvider_HealsLegacyRef(t *testing.T) {
	ctx := context.Background()

	legacy, err := NewFSBundleStore(t.TempDir())
	if err != nil {
		t.Fatalf("new fs store: %v", err)
	}
	files := map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")}
	legacyRef, err := legacy.Write(ctx, BundleIdentity{}, "", files)
	if err != nil {
		t.Fatalf("legacy write: %v", err)
	}

	cli := newFakeGitClient()
	active := NewGitEntryBundleStore(cli)

	repo := newFakeEntryRepo()
	org := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 1)
	org.SourceRef = legacyRef
	repo.mustCreate(t, org)

	resolver := &CatalogProviderResolver{Entries: repo, Bundles: active, Legacy: legacy}
	got, version, err := resolver.ResolveProvider(ctx, "tenant-1", "yandex", 0)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	if string(got["manifest.yaml"]) != string(files["manifest.yaml"]) {
		t.Fatalf("files = %v, want %v", got, files)
	}

	healed, err := repo.Get(ctx, catalogpb.Level_LEVEL_ORG, "tenant-1", org.GetEntity().GetId())
	if err != nil {
		t.Fatalf("get org entry: %v", err)
	}
	if !IsGitCommitRef(healed.GetSourceRef()) {
		t.Fatalf("org entry source_ref = %q, want commit-pinned after healing", healed.GetSourceRef())
	}
}

// TestForkEntry_HealsLegacyInstanceRefBeforeForking proves ForkEntry's reuse
// of resolveSourceRef also heals a legacy ref: forking a LINKED row whose
// backing instance row still carries a bare FS-era ref must not fail
// against Bundles.Fork (which internally Reads sourceRef, hitting the same
// "missing git: scheme" rejection GetOrgProviderFiles used to) — and the
// fork itself becomes a real per-entry repo in the tenant's own org.
func TestForkEntry_HealsLegacyInstanceRefBeforeForking(t *testing.T) {
	ctx := context.Background()

	legacy, err := NewFSBundleStore(t.TempDir())
	if err != nil {
		t.Fatalf("new fs store: %v", err)
	}
	files := map[string][]byte{"manifest.yaml": []byte("name: yandex\n")}
	legacyRef, err := legacy.Write(ctx, BundleIdentity{}, "", files)
	if err != nil {
		t.Fatalf("legacy write: %v", err)
	}

	cli := newFakeGitClient()
	active := NewGitEntryBundleStore(cli)

	repo := newFakeEntryRepo()
	instance := instanceEntry(catalogpb.Kind_KIND_PROVIDER, "yandex", 1)
	instance.SourceRef = legacyRef
	repo.mustCreate(t, instance)

	svc := NewService(Deps{
		Entries:       repo,
		Bundles:       active,
		LegacyBundles: legacy,
		Check:         stubChecker(nil),
		Authn:         fakeAuthn{},
	})
	if err := svc.SeedOrgCatalog(ctx, "tenant-1"); err != nil {
		t.Fatalf("SeedOrgCatalog: %v", err)
	}
	orgEntries, _ := repo.List(ctx, catalogpb.Level_LEVEL_ORG, "tenant-1", catalogpb.Kind_KIND_PROVIDER)
	linked := orgEntries[0]

	forked, err := svc.ForkEntry(ctx, "tenant-1", linked.GetEntity().GetId())
	if err != nil {
		t.Fatalf("ForkEntry: %v", err)
	}
	if !IsGitCommitRef(forked.GetSourceRef()) {
		t.Fatalf("forked source_ref = %q, want commit-pinned", forked.GetSourceRef())
	}
	forkedOwner, _, _, err := DecodeGitCommitRef(forked.GetSourceRef())
	if err != nil {
		t.Fatalf("decode forked ref: %v", err)
	}
	if forkedOwner != gitrepo.TenantOrg("tenant-1") {
		t.Fatalf("forked repo owner = %q, want tenant org %q", forkedOwner, gitrepo.TenantOrg("tenant-1"))
	}
	got, err := active.Read(ctx, forked.GetSourceRef())
	if err != nil {
		t.Fatalf("read forked bundle: %v", err)
	}
	if string(got["manifest.yaml"]) != string(files["manifest.yaml"]) {
		t.Fatalf("forked content = %v, want %v", got, files)
	}
}
