//go:build integration

package apitoken_test

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
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/apitoken"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
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

// fixture bundles the system-under-test and the RBAC prerequisites a tenant-scoped
// request needs: a tenant, an account, and the account's TenantMember role.
type fixture struct {
	svc       *apitoken.ApiTokenService
	executor  exec.DB
	tenantID  *models.TenantId
	accountID *models.AccountId
	ctx       context.Context // carries an account caller with the seeded role
}

// newFixture builds the ApiTokenService over a fresh cloned DB and seeds the FK
// chain (account -> tenant -> tenant_member) so authz.Require passes.
func newFixture(t *testing.T, role models.TenantMember_Role) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()

	az := authz.New(log, executor)
	svc := apitoken.NewApiTokenService(log, executor, trm, az)

	ctx := context.Background()
	acctSvc := tenancy.NewAccountAdminService(log, executor, trm)
	tenantSvc := tenancy.NewTenantAdminService(log, executor, trm)

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

	// Seed the membership row directly: AddMemberToTenant itself requires OWNER,
	// which is circular for the first member.
	memberRepo := repository.NewProtoRepository(
		repository.NewScannerRepository(models.TenantMembers.Table, executor),
		models.TenantMemberConverter,
	)
	member := &models.TenantMember{
		Entity:    ids.NewEntity(),
		TenantId:  tenantID,
		AccountId: accountID,
		Role:      role,
	}
	_, err = memberRepo.Execute(ctx, models.TenantMembers.Insert().From(member.IntoPlain().AllSetters()...))
	require.NoError(t, err)

	callerCtx := caller.NewContext(ctx, &caller.Caller{
		Kind:      caller.PrincipalAccount,
		AccountID: accountID,
	})

	return &fixture{
		svc:       svc,
		executor:  executor,
		tenantID:  tenantID,
		accountID: accountID,
		ctx:       callerCtx,
	}
}

func TestApiTokenCRUDRoundTrip(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_OWNER)

	// Create mints a token, returns the plaintext secret once.
	created, err := f.svc.CreateApiToken(f.ctx, &uipb.CreateApiTokenRequest{
		TenantId: f.tenantID,
		Name:     "ci-token",
		Role:     models.TenantMember_ROLE_ADMIN,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.GetSecret(), "plaintext secret returned once")

	tok := created.GetToken()
	id := tok.GetEntity().GetId().GetValue()
	require.NotEmpty(t, id)
	require.Equal(t, "ci-token", tok.GetName())
	require.Equal(t, models.TenantMember_ROLE_ADMIN, tok.GetRole())
	require.Equal(t, f.accountID.GetValue(), tok.GetOwned().GetOwnerAccountId().GetValue())
	require.Equal(t, f.tenantID.GetValue(), tok.GetOwned().GetTenantId().GetValue())

	// Only the sha256 hash is persisted; verify it matches HashToken(secret) and the
	// plaintext is nowhere in the row (read the hash via the scanner).
	hashScanner := repository.NewProtoRepository(
		repository.NewScannerRepository(models.ApiTokens.Table, f.executor),
		models.ApiTokenConverter,
	)
	row, err := hashScanner.Scanner().QueryRow(context.Background(),
		models.ApiTokens.SelectAll().Where(models.ApiTokens.Id.Eq(id)))
	require.NoError(t, err)
	require.Equal(t, domainauth.HashToken(created.GetSecret()), row.TokenHash)
	require.NotEqual(t, created.GetSecret(), row.TokenHash, "stored value is a hash, not the plaintext")

	// List returns the token (hash never exposed on the proto).
	list, err := f.svc.ListApiTokens(f.ctx, &uipb.ListApiTokensRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Len(t, list.GetApiTokens(), 1)
	require.Equal(t, id, list.GetApiTokens()[0].GetEntity().GetId().GetValue())
	require.Equal(t, "ci-token", list.GetApiTokens()[0].GetName())

	// Revoke soft-deletes; List no longer returns it.
	_, err = f.svc.RevokeApiToken(f.ctx, &uipb.RevokeApiTokenRequest{
		TenantId: f.tenantID,
		Id:       &models.Ulid{Value: id},
	})
	require.NoError(t, err)

	list, err = f.svc.ListApiTokens(f.ctx, &uipb.ListApiTokensRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	require.Empty(t, list.GetApiTokens())
}

// Negative: ADMIN is below the OWNER-only requirement for minting tokens.
func TestApiTokenRBACDeniesAdmin(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_ADMIN)

	_, err := f.svc.CreateApiToken(f.ctx, &uipb.CreateApiTokenRequest{
		TenantId: f.tenantID,
		Name:     "ci-token",
		Role:     models.TenantMember_ROLE_VIEWER,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// Negative: an anonymous caller (no principal in ctx) is unauthenticated.
func TestApiTokenRBACDeniesAnonymous(t *testing.T) {
	f := newFixture(t, models.TenantMember_ROLE_OWNER)

	_, err := f.svc.ListApiTokens(context.Background(), &uipb.ListApiTokensRequest{TenantId: f.tenantID})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
