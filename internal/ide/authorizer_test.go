package ide

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

type fakeTokenVerifier struct {
	claims map[string]*iampb.AccessClaims // token -> claims
}

func (f fakeTokenVerifier) Verify(_ context.Context, token string) (*iampb.AccessClaims, error) {
	c, ok := f.claims[token]
	if !ok {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

type fakePermResolver struct {
	// perms[accountID+"|"+tenantID]
	perms map[string][]*iampb.Permission
}

func (f fakePermResolver) EffectivePermissions(_ context.Context, accountID, tenantID string) ([]*iampb.Permission, error) {
	return f.perms[accountID+"|"+tenantID], nil
}

type fakeTenantResolver struct {
	bySlug map[string]*iampb.Tenant
}

func (f fakeTenantResolver) GetBySlug(_ context.Context, slug string) (*iampb.Tenant, error) {
	t, ok := f.bySlug[slug]
	if !ok {
		return nil, errors.New("tenant not found")
	}
	return t, nil
}

func req(path, bearer string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	return r
}

func TestAuthorizer_InstanceScope_RequiresPlatformAdmin(t *testing.T) {
	a := &Authorizer{
		Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{
			"admin-tok": {AccountId: "acc-1", IsAdmin: true},
			"user-tok":  {AccountId: "acc-2", IsAdmin: false},
		}},
	}
	if !a.CanAuthor(req("/ide/instance/", "admin-tok")) {
		t.Fatal("platform admin must be able to author the instance repo")
	}
	if a.CanAuthor(req("/ide/instance/", "user-tok")) {
		t.Fatal("non-admin must NOT be able to author the instance repo")
	}
}

func TestAuthorizer_OrgScope_RequiresAuthoringPermissionInThatTenant(t *testing.T) {
	tenants := fakeTenantResolver{bySlug: map[string]*iampb.Tenant{
		"acme": {Id: "tenant-acme"},
		"beta": {Id: "tenant-beta"},
	}}
	perms := fakePermResolver{perms: map[string][]*iampb.Permission{
		"acc-1|tenant-acme": {{Resource: iampb.Resource_RESOURCE_PROVIDER, Action: iampb.Action_ACTION_UPDATE}},
		// acc-1 has NO permissions in tenant-beta.
	}}
	a := &Authorizer{
		Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{
			"acc-1-tok": {AccountId: "acc-1"},
		}},
		Perms:   perms,
		Tenants: tenants,
	}
	if !a.CanAuthor(req("/ide/org/acme/", "acc-1-tok")) {
		t.Fatal("caller with RESOURCE_PROVIDER UPDATE in tenant-acme must be able to author org acme")
	}
	if a.CanAuthor(req("/ide/org/beta/", "acc-1-tok")) {
		t.Fatal("caller with no membership/permission in tenant-beta must NOT be able to author org beta — cross-tenant leak")
	}
}

func TestAuthorizer_OrgScope_ReadOnlyPermissionIsNotSufficient(t *testing.T) {
	tenants := fakeTenantResolver{bySlug: map[string]*iampb.Tenant{"acme": {Id: "tenant-acme"}}}
	perms := fakePermResolver{perms: map[string][]*iampb.Permission{
		"acc-1|tenant-acme": {{Resource: iampb.Resource_RESOURCE_PROVIDER, Action: iampb.Action_ACTION_READ}},
	}}
	a := &Authorizer{
		Tokens:  fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"tok": {AccountId: "acc-1"}}},
		Perms:   perms,
		Tenants: tenants,
	}
	if a.CanAuthor(req("/ide/org/acme/", "tok")) {
		t.Fatal("read-only permission must not grant IDE authoring access")
	}
}

func TestAuthorizer_PlatformAdmin_CanAuthorAnyOrg(t *testing.T) {
	tenants := fakeTenantResolver{bySlug: map[string]*iampb.Tenant{"acme": {Id: "tenant-acme"}}}
	a := &Authorizer{
		Tokens:  fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"admin-tok": {AccountId: "acc-1", IsAdmin: true}}},
		Perms:   fakePermResolver{},
		Tenants: tenants,
	}
	if !a.CanAuthor(req("/ide/org/acme/", "admin-tok")) {
		t.Fatal("platform admin must be able to author any org")
	}
}

func TestAuthorizer_FailsClosedOnMissingOrInvalidToken(t *testing.T) {
	a := &Authorizer{Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{}}}
	if a.CanAuthor(req("/ide/instance/", "")) {
		t.Fatal("missing bearer token must be rejected")
	}
	if a.CanAuthor(req("/ide/instance/", "garbage")) {
		t.Fatal("invalid bearer token must be rejected")
	}
}

func TestAuthorizer_FailsClosedOnUnparsableScope(t *testing.T) {
	a := &Authorizer{Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"tok": {AccountId: "acc-1", IsAdmin: true}}}}
	if a.CanAuthor(req("/ide/", "tok")) {
		t.Fatal("an unparsable scope must be rejected even for an admin token")
	}
}

func TestAuthorizer_FailsClosedOnUnknownOrgSlug(t *testing.T) {
	a := &Authorizer{
		Tokens:  fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"tok": {AccountId: "acc-1"}}},
		Perms:   fakePermResolver{},
		Tenants: fakeTenantResolver{bySlug: map[string]*iampb.Tenant{}},
	}
	if a.CanAuthor(req("/ide/org/nonexistent/", "tok")) {
		t.Fatal("an unknown org slug must be rejected")
	}
}
