package iam

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestGetTenantBySlugAllowsMember(t *testing.T) {
	svc := NewIamService(IamDeps{
		Authn:       fakeIamAuthn{claims: &iampb.AccessClaims{AccountId: "account-1"}},
		Tenants:     fakeTenantRepo{tenant: tenant("tenant-1", "acme")},
		Memberships: fakeMembershipRepo{member: true},
	})

	resp, err := svc.GetTenant(context.Background(), &api.GetTenantRequest{
		Ref: &api.GetTenantRequest_Slug{Slug: "acme"},
	})
	if err != nil {
		t.Fatalf("get tenant by slug: %v", err)
	}
	if got := resp.GetTenant().GetId(); got != "tenant-1" {
		t.Fatalf("tenant id = %q, want tenant-1", got)
	}
}

func TestGetTenantBySlugRejectsNonMember(t *testing.T) {
	svc := NewIamService(IamDeps{
		Authn:       fakeIamAuthn{claims: &iampb.AccessClaims{AccountId: "account-1"}},
		Tenants:     fakeTenantRepo{tenant: tenant("tenant-1", "acme")},
		Memberships: fakeMembershipRepo{},
	})

	_, err := svc.GetTenant(context.Background(), &api.GetTenantRequest{
		Ref: &api.GetTenantRequest_Slug{Slug: "acme"},
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.PermissionDenied, err)
	}
}

func TestGetTenantBySlugAllowsAdminWithoutMembership(t *testing.T) {
	svc := NewIamService(IamDeps{
		Authn: fakeIamAuthn{claims: &iampb.AccessClaims{
			AccountId: "admin-1",
			IsAdmin:   true,
		}},
		Tenants:     fakeTenantRepo{tenant: tenant("tenant-1", "acme")},
		Memberships: fakeMembershipRepo{},
	})

	resp, err := svc.GetTenant(context.Background(), &api.GetTenantRequest{
		Ref: &api.GetTenantRequest_Slug{Slug: "acme"},
	})
	if err != nil {
		t.Fatalf("get tenant by slug: %v", err)
	}
	if got := resp.GetTenant().GetId(); got != "tenant-1" {
		t.Fatalf("tenant id = %q, want tenant-1", got)
	}
}

func TestGetAccountAllowsCoTenantMember(t *testing.T) {
	svc := NewIamService(IamDeps{
		Authn: fakeIamAuthn{claims: &iampb.AccessClaims{AccountId: "account-1"}},
		Accounts: fakeAccountRepo{account: &iampb.Account{
			Id:       "account-2",
			Email:    "member@example.com",
			Nickname: "member",
		}},
		Memberships: fakeMembershipRepo{shared: true},
	})

	resp, err := svc.GetAccount(context.Background(), &api.GetAccountRequest{Id: "account-2"})
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got := resp.GetAccount().GetEmail(); got != "member@example.com" {
		t.Fatalf("email = %q, want member@example.com", got)
	}
}

func TestGetAccountRejectsUnrelatedAccount(t *testing.T) {
	svc := NewIamService(IamDeps{
		Authn: fakeIamAuthn{claims: &iampb.AccessClaims{AccountId: "account-1"}},
		Accounts: fakeAccountRepo{account: &iampb.Account{
			Id:       "account-2",
			Email:    "stranger@example.com",
			Nickname: "stranger",
		}},
		Memberships: fakeMembershipRepo{},
	})

	_, err := svc.GetAccount(context.Background(), &api.GetAccountRequest{Id: "account-2"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.PermissionDenied, err)
	}
}

func TestUpdateAccountRejectsNonSelfNonAdmin(t *testing.T) {
	svc := NewIamService(IamDeps{
		Authn: fakeIamAuthn{claims: &iampb.AccessClaims{AccountId: "account-1"}},
		Accounts: fakeAccountRepo{account: &iampb.Account{
			Id:       "account-2",
			Email:    "stranger@example.com",
			Nickname: "stranger",
		}},
	})

	_, err := svc.UpdateAccount(context.Background(), &api.UpdateAccountRequest{Id: "account-2"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.PermissionDenied, err)
	}
}

func TestCreateMembershipRejectsMissingAccount(t *testing.T) {
	svc := NewIamService(IamDeps{
		Accounts: fakeAccountRepo{},
		Tenants:  fakeTenantRepo{tenant: tenant("tenant-1", "acme")},
	})

	_, err := svc.CreateMembership(context.Background(), &api.CreateMembershipRequest{
		AccountId: "missing-account",
		TenantId:  "tenant-1",
		RoleIds:   []string{"role-1"},
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.NotFound, err)
	}
}

func TestDeleteMembershipRejectsTenantOwner(t *testing.T) {
	svc := NewIamService(IamDeps{
		Tenants: fakeTenantRepo{tenant: &iampb.Tenant{
			Id:             "tenant-1",
			Slug:           "acme",
			Name:           "acme",
			OwnerAccountId: "owner-1",
		}},
		Memberships: fakeMembershipRepo{
			membership: &iampb.Membership{
				Id:        "membership-1",
				AccountId: "owner-1",
				TenantId:  "tenant-1",
			},
		},
	})

	_, err := svc.DeleteMembership(context.Background(), &api.DeleteMembershipRequest{Id: "membership-1"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.FailedPrecondition, err)
	}
}

func TestDeleteRoleRejectsAssignedRole(t *testing.T) {
	svc := NewIamService(IamDeps{
		Roles: fakeRoleRepo{role: &iampb.Role{
			Id:       "role-1",
			Name:     "custom",
			Scope:    iampb.Scope_SCOPE_TENANT,
			TenantId: "tenant-1",
		}},
		Memberships: fakeMembershipRepo{roleInUse: true},
	})

	_, err := svc.DeleteRole(context.Background(), &api.DeleteRoleRequest{Id: "role-1"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status = %s, want %s (err %v)", status.Code(err), codes.FailedPrecondition, err)
	}
}

func tenant(id, slug string) *iampb.Tenant {
	return &iampb.Tenant{
		Id:             id,
		Slug:           slug,
		Name:           slug,
		OwnerAccountId: "owner-1",
	}
}

type fakeIamAuthn struct {
	claims *iampb.AccessClaims
}

func (f fakeIamAuthn) Caller(context.Context) (*iampb.AccessClaims, error) {
	return f.claims, nil
}

type fakeAccountRepo struct {
	account *iampb.Account
}

func (f fakeAccountRepo) Create(context.Context, *iampb.Account) error { return nil }
func (f fakeAccountRepo) Get(_ context.Context, id string) (*iampb.Account, error) {
	if f.account != nil && f.account.GetId() == id {
		return f.account, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeAccountRepo) GetByEmail(_ context.Context, email string) (*iampb.Account, error) {
	if f.account != nil && f.account.GetEmail() == email {
		return f.account, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeAccountRepo) GetByNickname(_ context.Context, nickname string) (*iampb.Account, error) {
	if f.account != nil && f.account.GetNickname() == nickname {
		return f.account, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeAccountRepo) List(context.Context, uint32, string) ([]*iampb.Account, string, error) {
	return nil, "", nil
}
func (f fakeAccountRepo) Update(_ context.Context, account *iampb.Account) error {
	f.account = account
	return nil
}
func (f fakeAccountRepo) Delete(context.Context, string) error { return nil }

type fakeRoleRepo struct {
	role *iampb.Role
}

func (f fakeRoleRepo) Create(context.Context, *iampb.Role) error { return nil }
func (f fakeRoleRepo) Get(_ context.Context, id string) (*iampb.Role, error) {
	if f.role != nil && f.role.GetId() == id {
		return f.role, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeRoleRepo) GetMany(context.Context, []string) ([]*iampb.Role, error) {
	if f.role != nil {
		return []*iampb.Role{f.role}, nil
	}
	return nil, nil
}
func (f fakeRoleRepo) List(context.Context, string, uint32, string) ([]*iampb.Role, string, error) {
	return nil, "", nil
}
func (f fakeRoleRepo) Update(context.Context, *iampb.Role) error { return nil }
func (f fakeRoleRepo) Delete(context.Context, string) error      { return nil }
func (f fakeRoleRepo) DeleteByTenant(context.Context, string) error {
	return nil
}

type fakeTenantRepo struct {
	tenant  *iampb.Tenant
	tenants []*iampb.Tenant
}

func (f fakeTenantRepo) Create(context.Context, *iampb.Tenant) error { return nil }
func (f fakeTenantRepo) Get(_ context.Context, id string) (*iampb.Tenant, error) {
	if f.tenant != nil && f.tenant.GetId() == id {
		return f.tenant, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeTenantRepo) GetBySlug(_ context.Context, slug string) (*iampb.Tenant, error) {
	if f.tenant != nil && f.tenant.GetSlug() == slug {
		return f.tenant, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeTenantRepo) ListByMember(context.Context, string) ([]*iampb.Tenant, error) {
	return f.tenants, nil
}
func (f fakeTenantRepo) CountOwnedBy(context.Context, string) (int, error) { return 0, nil }
func (f fakeTenantRepo) Update(context.Context, *iampb.Tenant) error       { return nil }
func (f fakeTenantRepo) Delete(context.Context, string) error              { return nil }

type fakeMembershipRepo struct {
	member     bool
	shared     bool
	roleInUse  bool
	membership *iampb.Membership
}

func (f fakeMembershipRepo) Create(context.Context, *iampb.Membership) error { return nil }
func (f fakeMembershipRepo) Get(context.Context, string) (*iampb.Membership, error) {
	if f.membership != nil {
		return f.membership, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeMembershipRepo) GetByAccountTenant(_ context.Context, accountID, tenantID string) (*iampb.Membership, error) {
	if f.member {
		return &iampb.Membership{Id: "membership-1", AccountId: accountID, TenantId: tenantID}, nil
	}
	return nil, derrors.ErrNotFound
}
func (f fakeMembershipRepo) ShareTenant(context.Context, string, string) (bool, error) {
	return f.shared, nil
}
func (f fakeMembershipRepo) RoleInUse(context.Context, string) (bool, error) {
	return f.roleInUse, nil
}
func (f fakeMembershipRepo) List(context.Context, string, uint32, string) ([]*iampb.Membership, string, error) {
	return nil, "", nil
}
func (f fakeMembershipRepo) Update(context.Context, *iampb.Membership) error { return nil }
func (f fakeMembershipRepo) Delete(context.Context, string) error            { return nil }
func (f fakeMembershipRepo) DeleteByTenant(context.Context, string) error    { return nil }
func (f fakeMembershipRepo) DeleteByAccount(context.Context, string) error   { return nil }
