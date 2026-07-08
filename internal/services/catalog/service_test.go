package catalog

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

/*
	===== CreateOrgProvider / CreateOrgWorkflow =====
*/

func TestCreateOrgProvider_NativeVersion1(t *testing.T) {
	repo := newFakeEntryRepo()
	svc := NewService(Deps{
		Entries: repo,
		Bundles: NewMemoryBundleStore(),
		Check:   stubChecker(nil),
		Authn:   fakeAuthn{},
	})
	resp, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1",
		Slug:     "yandex",
		Name:     "Yandex Cloud",
		Files:    map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("CreateOrgProvider: %v", err)
	}
	if resp.GetEntry().GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", resp.GetEntry().GetVersion())
	}
	if resp.GetEntry().GetOrigin() != catalogpb.Origin_ORIGIN_NATIVE {
		t.Fatalf("origin = %v, want NATIVE", resp.GetEntry().GetOrigin())
	}
	if resp.GetEntry().GetLevel() != catalogpb.Level_LEVEL_ORG {
		t.Fatalf("level = %v, want LEVEL_ORG", resp.GetEntry().GetLevel())
	}

	// second Create at the same slug bumps version
	resp2, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex Cloud v2",
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("CreateOrgProvider (2nd): %v", err)
	}
	if resp2.GetEntry().GetVersion() != 2 {
		t.Fatalf("version = %d, want 2", resp2.GetEntry().GetVersion())
	}
}

func TestCreateOrgProvider_RequiresTenantID(t *testing.T) {
	svc := NewService(Deps{Entries: newFakeEntryRepo(), Bundles: NewMemoryBundleStore(), Check: stubChecker(nil), Authn: fakeAuthn{}})
	_, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{Slug: "yandex"})
	if err == nil {
		t.Fatal("expected error for empty tenant_id")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

func TestCreateOrgWorkflow_NativeVersion1(t *testing.T) {
	svc := newTestService()
	resp, err := svc.CreateOrgWorkflow(context.Background(), &catalogpb.CreateOrgWorkflowRequest{
		TenantId: "tenant-1",
		Slug:     "pg-ha",
		Name:     "PG HA",
		Files:    map[string][]byte{"cluster.yaml": clusterYAML()},
	})
	if err != nil {
		t.Fatalf("CreateOrgWorkflow: %v", err)
	}
	entry := resp.GetEntry()
	if entry.GetKind() != catalogpb.Kind_KIND_WORKFLOW {
		t.Fatalf("kind = %v, want KIND_WORKFLOW", entry.GetKind())
	}
	if entry.GetSummary().GetProviderSlug() != "yandex" {
		t.Fatalf("summary.provider_slug = %q, want yandex", entry.GetSummary().GetProviderSlug())
	}
	if entry.GetEntity().GetAuthorId() != "account-1" {
		t.Fatalf("author_id = %q, want account-1", entry.GetEntity().GetAuthorId())
	}
}

func TestCreateOrgEntry_SummaryReflectsChecker(t *testing.T) {
	repo := newFakeEntryRepo()
	diags := []*dslpb.Diagnostic{{Severity: dslpb.Severity_SEVERITY_ERROR, Message: "boom"}}
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(diags), Authn: fakeAuthn{}})

	resp, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex",
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")},
	})
	if err != nil {
		t.Fatalf("CreateOrgProvider: %v", err)
	}
	if resp.GetEntry().GetSummary().GetCompiles() {
		t.Fatal("summary.compiles = true, want false (stub checker returned a diagnostic)")
	}
}

/*
	===== GetOrgProvider / GetOrgWorkflow (incl. kind-routing) =====
*/

func TestGetOrgProvider_Roundtrip(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	resp, err := svc.GetOrgProvider(context.Background(), &catalogpb.GetOrgProviderRequest{TenantId: "tenant-1", Id: id})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.GetEntry().GetEntity().GetId() != id {
		t.Fatal("returned a different entry")
	}
}

func TestGetOrgProvider_WrongKindIsNotFound(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgWorkflow(context.Background(), &catalogpb.CreateOrgWorkflowRequest{
		TenantId: "tenant-1", Slug: "pg-ha", Name: "PG HA", Files: map[string][]byte{"cluster.yaml": clusterYAML()},
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	// A caller with only RESOURCE_PROVIDER grants must not be able to reach
	// a RESOURCE_WORKFLOW entry through GetOrgProvider — kind-routing.
	_, err = svc.GetOrgProvider(context.Background(), &catalogpb.GetOrgProviderRequest{TenantId: "tenant-1", Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestGetOrgProvider_CrossTenantIsNotFound(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	_, err = svc.GetOrgProvider(context.Background(), &catalogpb.GetOrgProviderRequest{TenantId: "tenant-2", Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestGetOrgProvider_RequiresTenantID(t *testing.T) {
	svc := newTestService()
	_, err := svc.GetOrgProvider(context.Background(), &catalogpb.GetOrgProviderRequest{Id: "some-id"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

/*
	===== ListOrgProviders / ListOrgWorkflows =====
*/

func TestListOrgProviders_ScopesByTenantAndKind(t *testing.T) {
	svc := newTestService()
	for _, tc := range []struct{ tenantID, slug string }{
		{"tenant-1", "yandex"},
		{"tenant-1", "azure"},
		{"tenant-2", "yandex"},
	} {
		if _, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
			TenantId: tc.tenantID, Slug: tc.slug, Name: tc.slug, Files: providerFiles(),
		}); err != nil {
			t.Fatalf("create provider for %s/%s: %v", tc.tenantID, tc.slug, err)
		}
	}
	if _, err := svc.CreateOrgWorkflow(context.Background(), &catalogpb.CreateOrgWorkflowRequest{
		TenantId: "tenant-1", Slug: "pg-ha", Name: "PG HA", Files: map[string][]byte{"cluster.yaml": clusterYAML()},
	}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	resp, err := svc.ListOrgProviders(context.Background(), &catalogpb.ListOrgProvidersRequest{TenantId: "tenant-1"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.GetEntries()) != 2 {
		t.Fatalf("entries = %d, want 2 (only tenant-1's providers, not its workflow or tenant-2's provider)", len(resp.GetEntries()))
	}
}

/*
	===== UpdateOrgProvider / UpdateOrgWorkflow =====
*/

func TestUpdateOrgProvider_EditsFilesInPlaceKeepingVersion(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	resp, err := svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: id,
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n  - network\n")},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if resp.GetEntry().GetVersion() != created.GetEntry().GetVersion() {
		t.Fatalf("version changed on update: got %d, want unchanged %d", resp.GetEntry().GetVersion(), created.GetEntry().GetVersion())
	}
	if len(resp.GetEntry().GetSummary().GetProvides()) != 2 {
		t.Fatalf("provides = %v, want 2 entries reflecting the updated manifest", resp.GetEntry().GetSummary().GetProvides())
	}
}

func TestUpdateOrgProvider_WrongKindIsNotFound(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgWorkflow(context.Background(), &catalogpb.CreateOrgWorkflowRequest{
		TenantId: "tenant-1", Slug: "pg-ha", Name: "PG HA", Files: map[string][]byte{"cluster.yaml": clusterYAML()},
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	_, err = svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: id, Files: providerFiles(),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

// TestUpdateOrgProvider_LinkedEntryForksInsteadOfFailing exercises
// updateOrgEntry's fork-on-edit path against a LINKED row created by
// LinkInstanceProvider (which — unlike SeedOrgCatalog — copies the
// instance row's source_ref directly onto the LINKED row), complementing
// fork_test.go's SeedOrgCatalog-seeded coverage of ForkEntry's other
// resolution path (an empty own source_ref, read via source_entry_id).
// Before Task 6 this asserted FailedPrecondition; fork-on-edit supersedes
// that placeholder behavior.
func TestUpdateOrgProvider_LinkedEntryForksInsteadOfFailing(t *testing.T) {
	svc := newTestService()
	instance, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_PROVIDER, Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create instance entry: %v", err)
	}
	linked, err := svc.LinkInstanceProvider(context.Background(), &catalogpb.LinkInstanceProviderRequest{
		TenantId: "tenant-1", InstanceEntryId: instance.GetEntry().GetEntity().GetId(),
	})
	if err != nil {
		t.Fatalf("link: %v", err)
	}

	resp, err := svc.UpdateOrgProvider(context.Background(), &catalogpb.UpdateOrgProviderRequest{
		TenantId: "tenant-1", Id: linked.GetEntry().GetEntity().GetId(),
		Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n  - volumes\n")},
	})
	if err != nil {
		t.Fatalf("UpdateOrgProvider: %v", err)
	}
	if resp.GetEntry().GetOrigin() != catalogpb.Origin_ORIGIN_FORKED {
		t.Fatalf("origin = %v, want FORKED", resp.GetEntry().GetOrigin())
	}
	if resp.GetEntry().GetEntity().GetId() == linked.GetEntry().GetEntity().GetId() {
		t.Fatal("fork-on-edit must produce a new row id, not mutate the LINKED row in place")
	}
	if resp.GetEntry().GetSourceRef() == "" || resp.GetEntry().GetSourceRef() == instance.GetEntry().GetSourceRef() {
		t.Fatal("forked row must have its own distinct source_ref")
	}
}

/*
	===== DeleteOrgProvider / DeleteOrgWorkflow =====
*/

func TestDeleteOrgProvider_ThenGetIsNotFound(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgProvider(context.Background(), &catalogpb.CreateOrgProviderRequest{
		TenantId: "tenant-1", Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	if _, err := svc.DeleteOrgProvider(context.Background(), &catalogpb.DeleteOrgProviderRequest{TenantId: "tenant-1", Id: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = svc.GetOrgProvider(context.Background(), &catalogpb.GetOrgProviderRequest{TenantId: "tenant-1", Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestDeleteOrgProvider_AbsentIdIsIdempotent(t *testing.T) {
	svc := newTestService()
	if _, err := svc.DeleteOrgProvider(context.Background(), &catalogpb.DeleteOrgProviderRequest{TenantId: "tenant-1", Id: "missing"}); err != nil {
		t.Fatalf("delete absent: %v, want nil (idempotent)", err)
	}
}

func TestDeleteOrgProvider_WrongKindDoesNotDeleteWorkflow(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateOrgWorkflow(context.Background(), &catalogpb.CreateOrgWorkflowRequest{
		TenantId: "tenant-1", Slug: "pg-ha", Name: "PG HA", Files: map[string][]byte{"cluster.yaml": clusterYAML()},
	})
	if err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	if _, err := svc.DeleteOrgProvider(context.Background(), &catalogpb.DeleteOrgProviderRequest{TenantId: "tenant-1", Id: id}); err != nil {
		t.Fatalf("delete via wrong-kind RPC: %v, want nil (idempotent no-op)", err)
	}
	// The workflow must still be reachable through its own kind's Get.
	if _, err := svc.GetOrgWorkflow(context.Background(), &catalogpb.GetOrgWorkflowRequest{TenantId: "tenant-1", Id: id}); err != nil {
		t.Fatalf("workflow was deleted by the provider-kind Delete call: %v", err)
	}
}

/*
	===== Instance-level CRUD (admin_only) =====
*/

func TestCreateInstanceEntry_PersistsWithoutTenant(t *testing.T) {
	svc := newTestService()
	resp, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_PROVIDER, Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create instance entry: %v", err)
	}
	entry := resp.GetEntry()
	if entry.GetLevel() != catalogpb.Level_LEVEL_INSTANCE {
		t.Fatalf("level = %v, want LEVEL_INSTANCE", entry.GetLevel())
	}
	if entry.GetEntity().GetTenantId() != "" {
		t.Fatalf("tenant_id = %q, want empty", entry.GetEntity().GetTenantId())
	}
	if entry.GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", entry.GetVersion())
	}
}

func TestInstanceEntry_GetUpdateDeleteRoundtrip(t *testing.T) {
	svc := newTestService()
	created, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_PROVIDER, Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.GetEntry().GetEntity().GetId()

	got, err := svc.GetInstanceEntry(context.Background(), &catalogpb.GetInstanceEntryRequest{Id: id})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetEntry().GetEntity().GetId() != id {
		t.Fatal("get returned a different entry")
	}

	updated, err := svc.UpdateInstanceEntry(context.Background(), &catalogpb.UpdateInstanceEntryRequest{
		Id: id, Files: map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n  - network\n")},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated.GetEntry().GetSummary().GetProvides()) != 2 {
		t.Fatalf("provides = %v, want 2 entries after update", updated.GetEntry().GetSummary().GetProvides())
	}

	if _, err := svc.DeleteInstanceEntry(context.Background(), &catalogpb.DeleteInstanceEntryRequest{Id: id}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = svc.GetInstanceEntry(context.Background(), &catalogpb.GetInstanceEntryRequest{Id: id})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

func TestListInstanceEntries_FiltersByKind(t *testing.T) {
	svc := newTestService()
	if _, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_PROVIDER, Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_WORKFLOW, Slug: "pg-ha", Name: "PG HA", Files: map[string][]byte{"cluster.yaml": clusterYAML()},
	}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}

	resp, err := svc.ListInstanceEntries(context.Background(), &catalogpb.ListInstanceEntriesRequest{Kind: catalogpb.Kind_KIND_PROVIDER})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.GetEntries()) != 1 {
		t.Fatalf("entries = %d, want 1", len(resp.GetEntries()))
	}
	if resp.GetEntries()[0].GetKind() != catalogpb.Kind_KIND_PROVIDER {
		t.Fatalf("kind = %v, want KIND_PROVIDER", resp.GetEntries()[0].GetKind())
	}
}

/*
	===== LinkInstanceProvider / LinkInstanceWorkflow =====
*/

func TestLinkInstanceProvider_CreatesLinkedOrgEntry(t *testing.T) {
	svc := newTestService()
	instance, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_PROVIDER, Slug: "yandex", Name: "Yandex", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("create instance entry: %v", err)
	}
	instanceID := instance.GetEntry().GetEntity().GetId()

	resp, err := svc.LinkInstanceProvider(context.Background(), &catalogpb.LinkInstanceProviderRequest{
		TenantId: "tenant-1", InstanceEntryId: instanceID,
	})
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	entry := resp.GetEntry()
	if entry.GetOrigin() != catalogpb.Origin_ORIGIN_LINKED {
		t.Fatalf("origin = %v, want ORIGIN_LINKED", entry.GetOrigin())
	}
	if entry.GetLevel() != catalogpb.Level_LEVEL_ORG {
		t.Fatalf("level = %v, want LEVEL_ORG", entry.GetLevel())
	}
	if entry.GetSourceEntryId() != instanceID {
		t.Fatalf("source_entry_id = %q, want %q", entry.GetSourceEntryId(), instanceID)
	}
	if entry.GetSlug() != "yandex" {
		t.Fatalf("slug = %q, want yandex (inherited from the instance entry)", entry.GetSlug())
	}
	if entry.GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", entry.GetVersion())
	}
}

func TestLinkInstanceProvider_WrongKindSourceIsNotFound(t *testing.T) {
	svc := newTestService()
	instance, err := svc.CreateInstanceEntry(context.Background(), &catalogpb.CreateInstanceEntryRequest{
		Kind: catalogpb.Kind_KIND_WORKFLOW, Slug: "pg-ha", Name: "PG HA", Files: map[string][]byte{"cluster.yaml": clusterYAML()},
	})
	if err != nil {
		t.Fatalf("create instance entry: %v", err)
	}

	_, err = svc.LinkInstanceProvider(context.Background(), &catalogpb.LinkInstanceProviderRequest{
		TenantId: "tenant-1", InstanceEntryId: instance.GetEntry().GetEntity().GetId(),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.NotFound, err)
	}
}

/*
	===== CheckCatalogProvider / CheckCatalogWorkflow =====
*/

func TestCheckCatalogProvider_ReturnsDiagnosticsWithoutPersisting(t *testing.T) {
	repo := newFakeEntryRepo()
	diags := []*dslpb.Diagnostic{{Severity: dslpb.Severity_SEVERITY_WARNING, Message: "watch out"}}
	svc := NewService(Deps{Entries: repo, Bundles: NewMemoryBundleStore(), Check: stubChecker(diags), Authn: fakeAuthn{}})

	resp, err := svc.CheckCatalogProvider(context.Background(), &catalogpb.CheckCatalogProviderRequest{
		TenantId: "tenant-1", Files: providerFiles(),
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(resp.GetDiagnostics()) != 1 || resp.GetDiagnostics()[0].GetMessage() != "watch out" {
		t.Fatalf("diagnostics = %v, want the stub diagnostic", resp.GetDiagnostics())
	}
	if len(repo.byID) != 0 {
		t.Fatalf("repo has %d rows, want 0 (Check must never persist)", len(repo.byID))
	}
}

func TestCheckCatalogProvider_RequiresTenantID(t *testing.T) {
	svc := newTestService()
	_, err := svc.CheckCatalogProvider(context.Background(), &catalogpb.CheckCatalogProviderRequest{Files: providerFiles()})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

/*
	===== test doubles =====
*/

func newTestService() *Service {
	return NewService(Deps{
		Entries: newFakeEntryRepo(),
		Bundles: NewMemoryBundleStore(),
		Check:   stubChecker(nil),
		Authn:   fakeAuthn{},
	})
}

func providerFiles() map[string][]byte {
	return map[string][]byte{"manifest.yaml": []byte("name: yandex\nprovides:\n  - machines\n")}
}

func clusterYAML() []byte {
	return []byte("version: 1\nprovider:\n  use: yandex\nmachines:\n  db:\n    count: 1\nservices:\n  postgres: {}\n")
}

func stubChecker(diags []*dslpb.Diagnostic) Checker {
	return func(context.Context, catalogpb.Kind, map[string][]byte) ([]*dslpb.Diagnostic, error) {
		return diags, nil
	}
}

type fakeAuthn struct{}

func (fakeAuthn) Caller(context.Context) (*iam.AccessClaims, error) {
	return &iam.AccessClaims{AccountId: "account-1"}, nil
}

// fakeEntryRepo is an in-memory CatalogEntryRepo keyed by (level, tenantID,
// id), scanning for the slug/kind-scoped queries — small enough test data
// that a linear scan is simpler than maintaining a second index.
type fakeEntryRepo struct {
	byID map[string]*catalogpb.CatalogEntry
}

func newFakeEntryRepo() *fakeEntryRepo {
	return &fakeEntryRepo{byID: map[string]*catalogpb.CatalogEntry{}}
}

// mustCreate persists e via Create, failing the test on error, and returns e
// for chaining — a seed-test convenience so SeedOrgCatalog tests can set up
// pre-existing LEVEL_INSTANCE rows in one line.
func (r *fakeEntryRepo) mustCreate(t *testing.T, e *catalogpb.CatalogEntry) *catalogpb.CatalogEntry {
	t.Helper()
	if err := r.Create(context.Background(), e); err != nil {
		t.Fatalf("mustCreate: %v", err)
	}
	return e
}

// instanceEntry builds a minimal ORIGIN_NATIVE LEVEL_INSTANCE row for
// SeedOrgCatalog tests — it never goes through createEntry, so it carries no
// bundle/summary, which SeedOrgCatalog's tests never assert on.
func instanceEntry(kind catalogpb.Kind, slug string, version uint32) *catalogpb.CatalogEntry {
	return &catalogpb.CatalogEntry{
		Entity:  &common.Entity{Id: uuid.NewString(), Name: slug},
		Level:   catalogpb.Level_LEVEL_INSTANCE,
		Kind:    kind,
		Slug:    slug,
		Version: version,
		Origin:  catalogpb.Origin_ORIGIN_NATIVE,
	}
}

func entryKey(level catalogpb.Level, tenantID, id string) string {
	return fmt.Sprintf("%d|%s|%s", level, tenantID, id)
}

func (r *fakeEntryRepo) Create(_ context.Context, e *catalogpb.CatalogEntry) error {
	r.byID[entryKey(e.GetLevel(), e.GetEntity().GetTenantId(), e.GetEntity().GetId())] = e
	return nil
}

func (r *fakeEntryRepo) Get(_ context.Context, level catalogpb.Level, tenantID, id string) (*catalogpb.CatalogEntry, error) {
	e, ok := r.byID[entryKey(level, tenantID, id)]
	if !ok {
		return nil, derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	return e, nil
}

func (r *fakeEntryRepo) List(_ context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind) ([]*catalogpb.CatalogEntry, error) {
	var out []*catalogpb.CatalogEntry
	for _, e := range r.byID {
		if e.GetLevel() != level || e.GetEntity().GetTenantId() != tenantID {
			continue
		}
		if kind != catalogpb.Kind_KIND_UNSPECIFIED && e.GetKind() != kind {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *fakeEntryRepo) GetLatestBySlug(_ context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string) (*catalogpb.CatalogEntry, error) {
	var latest *catalogpb.CatalogEntry
	for _, e := range r.byID {
		if e.GetLevel() != level || e.GetEntity().GetTenantId() != tenantID || e.GetKind() != kind || e.GetSlug() != slug {
			continue
		}
		if latest == nil || e.GetVersion() > latest.GetVersion() {
			latest = e
		}
	}
	if latest == nil {
		return nil, derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	return latest, nil
}

func (r *fakeEntryRepo) GetBySlugVersion(_ context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug string, version uint32) (*catalogpb.CatalogEntry, error) {
	for _, e := range r.byID {
		if e.GetLevel() == level && e.GetEntity().GetTenantId() == tenantID && e.GetKind() == kind && e.GetSlug() == slug && e.GetVersion() == version {
			return e, nil
		}
	}
	return nil, derrors.NotFound("catalog_entry", "catalog entry not found")
}

func (r *fakeEntryRepo) ListBySource(_ context.Context, sourceEntryID string) ([]*catalogpb.CatalogEntry, error) {
	var out []*catalogpb.CatalogEntry
	for _, e := range r.byID {
		if e.GetSourceEntryId() == sourceEntryID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *fakeEntryRepo) Update(_ context.Context, e *catalogpb.CatalogEntry) error {
	key := entryKey(e.GetLevel(), e.GetEntity().GetTenantId(), e.GetEntity().GetId())
	if _, ok := r.byID[key]; !ok {
		return derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	r.byID[key] = e
	return nil
}

func (r *fakeEntryRepo) Delete(_ context.Context, level catalogpb.Level, tenantID, id string) error {
	key := entryKey(level, tenantID, id)
	if _, ok := r.byID[key]; !ok {
		return derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	delete(r.byID, key)
	return nil
}
