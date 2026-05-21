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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
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

func TestPresetRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := f.adminCtx()

	// Create.
	created, err := f.svc.CreatePreset(ctx, f.newPreset("alpha"))
	require.NoError(t, err)
	id := created.GetEntity().GetId().GetValue()
	require.NotEmpty(t, id)
	require.Equal(t, f.ownerID.GetValue(), created.GetOwned().GetOwnerAccountId().GetValue())
	require.Equal(t, f.tenantID.GetValue(), created.GetOwned().GetTenantId().GetValue())

	// List reads it back from the DB; persisted kind + tags must round-trip.
	list, err := f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)
	require.Len(t, list.GetPresets(), 1)
	got := list.GetPresets()[0]
	require.Equal(t, id, got.GetEntity().GetId().GetValue())
	require.Equal(t, models.Preset_KIND_DATABASE, got.GetKind())
	require.Equal(t, []string{"alpha"}, got.GetTags().GetTags())

	// List filtered by a different kind excludes it (Kind filter is honored).
	other, err := f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_WORKLOAD,
	})
	require.NoError(t, err)
	require.Empty(t, other.GetPresets())

	// Update mutable fields (tags); id + kind preserved.
	created.Tags = &commonpb.Tags{Tags: []string{"beta"}}
	updated, err := f.svc.UpdatePreset(ctx, created)
	require.NoError(t, err)
	require.Equal(t, id, updated.GetEntity().GetId().GetValue())
	require.Equal(t, []string{"beta"}, updated.GetTags().GetTags())

	// The update is persisted (read back via List).
	list, err = f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)
	require.Len(t, list.GetPresets(), 1)
	require.Equal(t, []string{"beta"}, list.GetPresets()[0].GetTags().GetTags())

	// Clone duplicates under a new id.
	cloned, err := f.svc.ClonePreset(ctx, &uipb.ClonePresetRequest{
		TenantId: f.tenantID,
		Id:       &models.DatabasePresetId{Value: id},
	})
	require.NoError(t, err)
	require.NotEqual(t, id, cloned.GetEntity().GetId().GetValue())
	require.Equal(t, []string{"beta"}, cloned.GetTags().GetTags())

	list, err = f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)
	require.Len(t, list.GetPresets(), 2)

	// Delete (soft) the original; only the clone remains.
	_, err = f.svc.DeletePreset(ctx, &uipb.DeletePresetRequest{
		TenantId: f.tenantID,
		Id:       &models.DatabasePresetId{Value: id},
	})
	require.NoError(t, err)

	list, err = f.svc.ListPresets(ctx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)
	require.Len(t, list.GetPresets(), 1)
	require.Equal(t, cloned.GetEntity().GetId().GetValue(), list.GetPresets()[0].GetEntity().GetId().GetValue())
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

	// A VIEWER API token may list (>= VIEWER) but not create (create needs ADMIN).
	viewerCtx := caller.NewContext(context.Background(), &caller.Caller{
		Kind:     caller.PrincipalApiToken,
		TenantID: f.tenantID,
		Role:     models.TenantMember_ROLE_VIEWER,
	})
	_, err = f.svc.ListPresets(viewerCtx, &uipb.ListPresetRequest{
		TenantId: f.tenantID,
		Kind:     models.Preset_KIND_DATABASE,
	})
	require.NoError(t, err)

	_, err = f.svc.CreatePreset(viewerCtx, f.newPreset("denied"))
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
