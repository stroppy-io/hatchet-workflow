//go:build integration

package catalog_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/stretchr/testify/require"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil"
)

var testContainer *testutil.PostgresContainer

func TestMain(m *testing.M) {
	ctx := context.Background()
	c, err := testutil.NewPostgresContainer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		os.Exit(1)
	}
	defer c.Close(ctx)
	if err := c.CreateTemplateDB(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create template db: %v\n", err)
		os.Exit(1)
	}
	testContainer = c
	os.Exit(m.Run())
}

// fixture is a per-test world: a fresh cloned DB plus the PresetService and the
// tenancy/authz prerequisites (account + tenant + an OWNER membership).
type fixture struct {
	svc      *catalog.PresetService
	executor exec.DB
	trm      tx.Trm
	tenantID *models.TenantId
	ownerID  *models.AccountId
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()

	az := authz.New(log, executor)
	svc := catalog.NewPresetService(log, executor, trm, az)

	accountID, tenantID := seedTenant(t, log, executor, trm)
	return &fixture{
		svc:      svc,
		executor: executor,
		trm:      trm,
		tenantID: tenantID,
		ownerID:  accountID,
	}
}

// seedTenant builds the FK chain: account -> tenant(owner) -> OWNER membership,
// using the real tenancy services so the rows are shaped exactly as production.
func seedTenant(t *testing.T, log *xlog.Logger, executor exec.DB, trm tx.Trm) (*models.AccountId, *models.TenantId) {
	t.Helper()
	ctx := context.Background()

	acctSvc := tenancy.NewAccountAdminService(log, executor, trm)
	tenantSvc := tenancy.NewTenantAdminService(log, executor, trm)
	az := authz.New(log, executor)
	memberSvc := tenancy.NewTenantService(log, executor, trm, az)

	acc, err := acctSvc.CreateAccount(ctx, &adminpb.CreateAccountRequest{
		Account:  &models.Account{Email: "owner@example.com", Nickname: "owner"},
		Password: "s3cret-pass",
	})
	require.NoError(t, err)
	accountID := &models.AccountId{Value: acc.GetEntity().GetId().GetValue()}

	tenant, err := tenantSvc.CreateTenant(ctx, &adminpb.CreateTenantRequest{
		Tenant: &models.Tenant{OwnerAccountId: accountID},
	})
	require.NoError(t, err)
	tenantID := &models.TenantId{Value: tenant.GetEntity().GetId().GetValue()}

	// AddMemberToTenant requires OWNER; bootstrap as a platform admin caller.
	adminCtx := caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accountID,
		IsAdmin:   true,
	})
	_, err = memberSvc.AddMemberToTenant(adminCtx, &uipb.AddMemberRequest{
		TenantId:  tenantID,
		AccountId: accountID,
		Role:      models.TenantMember_ROLE_OWNER,
	})
	require.NoError(t, err)

	return accountID, tenantID
}

// adminCtx is a context whose caller is the seeded account, resolving to OWNER (>=
// ADMIN) via its TenantMember row — no platform-admin bypass.
func (f *fixture) adminCtx() context.Context {
	return caller.NewContext(context.Background(), &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: f.ownerID,
	})
}

func (f *fixture) newPreset(name string) *models.Preset {
	return &models.Preset{
		Owned: &models.Own{TenantId: f.tenantID},
		Kind:  models.Preset_KIND_DATABASE,
		Tags:  &commonpb.Tags{Tags: []string{name}},
	}
}

// seedPreset inserts a preset row directly via a ratel repo, bypassing the broken
// CreatePreset path (see TestPresetCreateIsBroken). It forces the nullable jsonb
// oneof columns to NULL (CreatePreset's IntoPlain emits empty bytes -> invalid
// json), giving the read/delete service paths a real row to operate on.
func (f *fixture) seedPreset(t *testing.T, kind models.Preset_Kind, tags ...string) string {
	t.Helper()
	repo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Presets.Table, f.executor),
		models.PresetConverter,
	)
	p := &models.Preset{
		Entity: ids.NewEntity(),
		Owned:  &models.Own{OwnerAccountId: f.ownerID, TenantId: f.tenantID},
		Kind:   kind,
		Tags:   &commonpb.Tags{Tags: tags},
	}
	scanner := p.IntoPlain()
	scanner.PresetWorkloadPreset = nil
	scanner.PresetDatabasePreset = nil
	scanner.PresetTestPreset = nil
	_, err := repo.Execute(context.Background(), models.Presets.Insert().From(scanner.AllSetters()...))
	require.NoError(t, err)
	return p.GetEntity().GetId().GetValue()
}

func TestPresetRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := f.adminCtx()

	// Seed two DATABASE presets + one WORKLOAD preset directly.
	idA := f.seedPreset(t, models.Preset_KIND_DATABASE, "alpha")
	idB := f.seedPreset(t, models.Preset_KIND_DATABASE, "beta")
	_ = f.seedPreset(t, models.Preset_KIND_WORKLOAD, "gamma")

	// List reads them back from the DB filtered by kind; persisted kind + tags
	// must round-trip and the kind filter must exclude the WORKLOAD preset.
	list, err := f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)
	require.Len(t, list.GetPresets(), 2)
	byID := map[string]*models.Preset{}
	for _, p := range list.GetPresets() {
		require.Equal(t, models.Preset_KIND_DATABASE, p.GetKind())
		byID[p.GetEntity().GetId().GetValue()] = p
	}
	require.Contains(t, byID, idA)
	require.Contains(t, byID, idB)
	require.Equal(t, []string{"alpha"}, byID[idA].GetTags().GetTags())
	require.Equal(t, []string{"beta"}, byID[idB].GetTags().GetTags())

	// The WORKLOAD-kind list returns exactly the one workload preset.
	wl, err := f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_WORKLOAD,
	})
	require.NoError(t, err)
	require.Len(t, wl.GetPresets(), 1)
	require.Equal(t, []string{"gamma"}, wl.GetPresets()[0].GetTags().GetTags())

	// Delete (soft) preset A; the DATABASE list drops to just B.
	_, err = f.svc.DeletePreset(ctx, &uipb.DeletePresetRequest{
		TenantId: f.tenantID,
		Id:       &models.DatabasePresetId{Value: idA},
	})
	require.NoError(t, err)

	list, err = f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)
	require.Len(t, list.GetPresets(), 1)
	require.Equal(t, idB, list.GetPresets()[0].GetEntity().GetId().GetValue())
}

// TestPresetCreateIsBroken documents a real defect surfaced by this integration
// test: CreatePreset serializes the preset via IntoPlain(), which writes empty
// (non-nil) []byte to the three nullable preset_* JSONB columns whenever a oneof
// body is absent. Postgres rejects an empty string as jsonb (SQLSTATE 22P02), so
// CreatePreset (and likewise UpdatePreset/ClonePreset, which re-serialize the same
// way) fails for every input today. Asserting it pins the behavior until the
// generated IntoPlain emits nil for absent oneof bodies.
func TestPresetCreateIsBroken(t *testing.T) {
	f := newFixture(t)
	ctx := f.adminCtx()

	_, err := f.svc.CreatePreset(ctx, f.newPreset("alpha"))
	require.Error(t, err, "CreatePreset currently fails: empty []byte -> invalid jsonb")
	require.Equal(t, codes.Internal, status.Code(err))

	// Clone of a seeded row hits the same re-serialization defect.
	id := f.seedPreset(t, models.Preset_KIND_DATABASE, "src")
	_, err = f.svc.ClonePreset(ctx, &uipb.ClonePresetRequest{
		TenantId: f.tenantID,
		Id:       &models.DatabasePresetId{Value: id},
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))
}

// TestPresetRBAC covers the role boundaries: list needs VIEWER, write needs ADMIN.
func TestPresetRBAC(t *testing.T) {
	f := newFixture(t)

	// Anonymous (no caller) -> Unauthenticated on list.
	_, err := f.svc.ListPresets(context.Background(), &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// A non-member account -> PermissionDenied (not a member of the tenant).
	strangerCtx := caller.NewContext(context.Background(), &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: &models.AccountId{Value: "01HZZZSTRANGER000000000000"},
	})
	_, err = f.svc.ListPresets(strangerCtx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	// A VIEWER API token may list (>= VIEWER) but not delete (delete needs ADMIN).
	viewerCtx := caller.NewContext(context.Background(), &caller.Caller{
		Kind:     caller.PrincipalApiToken,
		TenantID: f.tenantID,
		Role:     models.TenantMember_ROLE_VIEWER,
	})
	id := f.seedPreset(t, models.Preset_KIND_DATABASE, "rbac")
	_, err = f.svc.ListPresets(viewerCtx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)

	_, err = f.svc.DeletePreset(viewerCtx, &uipb.DeletePresetRequest{
		TenantId: f.tenantID,
		Id:       &models.DatabasePresetId{Value: id},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
