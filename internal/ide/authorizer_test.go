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
	if !a.CanAuthor(req("/ide/instance/provider/docker/", "admin-tok")) {
		t.Fatal("platform admin must be able to author the instance repo")
	}
	if a.CanAuthor(req("/ide/instance/provider/docker/", "user-tok")) {
		t.Fatal("non-admin must NOT be able to author the instance repo")
	}
}

// TestAuthorizer_RecipeScope_InstanceLevelRejectedEvenForAdmin asserts
// EntryKindRecipe's documented invariant: unlike provider/workflow (where a
// platform admin CAN author the instance repo — see the test above), a
// recipe is always tenant-owned and has NO instance level at all — not even
// a platform admin credential may open "/ide/instance/recipe/...". This is
// the requirement, not merely today's behavior: a future refactor that
// accidentally let IsAdmin's short-circuit cover this path (the way it does
// for org scopes) would silently open a repo namespace this product
// decision says must not exist.
func TestAuthorizer_RecipeScope_InstanceLevelRejectedEvenForAdmin(t *testing.T) {
	a := &Authorizer{
		Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{
			"admin-tok": {AccountId: "acc-1", IsAdmin: true},
		}},
	}
	if a.CanAuthor(req("/ide/instance/recipe/pg-ha/", "admin-tok")) {
		t.Fatal("even a platform admin must NOT be able to author an instance-level recipe scope — recipes have no instance level")
	}
}

// TestAuthorizer_RecipeScope_CrossTenantDenied is the recipe analogue of
// TestAuthorizer_OrgScope_RequiresAuthoringPermissionInThatTenant: a caller
// holding RESOURCE_RECIPE authoring rights in tenant-acme must not be able
// to open tenant-beta's recipe repo by swapping the URL's org slug.
func TestAuthorizer_RecipeScope_CrossTenantDenied(t *testing.T) {
	tenants := fakeTenantResolver{bySlug: map[string]*iampb.Tenant{
		"acme": {Id: "tenant-acme"},
		"beta": {Id: "tenant-beta"},
	}}
	perms := fakePermResolver{perms: map[string][]*iampb.Permission{
		"acc-1|tenant-acme": {{Resource: iampb.Resource_RESOURCE_RECIPE, Action: iampb.Action_ACTION_UPDATE}},
		// acc-1 has NO permissions in tenant-beta.
	}}
	a := &Authorizer{
		Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{
			"acc-1-tok": {AccountId: "acc-1"},
		}},
		Perms:   perms,
		Tenants: tenants,
	}
	if !a.CanAuthor(req("/ide/org/acme/recipe/pg-ha/", "acc-1-tok")) {
		t.Fatal("caller with RESOURCE_RECIPE UPDATE in tenant-acme must be able to author acme's recipe entry")
	}
	if a.CanAuthor(req("/ide/org/beta/recipe/pg-ha/", "acc-1-tok")) {
		t.Fatal("caller with no membership/permission in tenant-beta must NOT be able to author beta's recipe repo — cross-tenant leak")
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
	if !a.CanAuthor(req("/ide/org/acme/provider/docker/", "acc-1-tok")) {
		t.Fatal("caller with RESOURCE_PROVIDER UPDATE in tenant-acme must be able to author org acme's provider entry")
	}
	if a.CanAuthor(req("/ide/org/beta/provider/docker/", "acc-1-tok")) {
		t.Fatal("caller with no membership/permission in tenant-beta must NOT be able to author org beta — cross-tenant leak")
	}
}

// TestAuthorizer_OrgScope_KindGrantDoesNotCrossOver proves the per-entry
// tightening this redesign makes: a caller who only holds RESOURCE_WORKFLOW
// authoring rights must NOT be able to open a RESOURCE_PROVIDER entry's
// repo (and vice versa) — the pre-redesign Authorizer granted either
// resource access to the whole shared org worktree; now a Scope names one
// specific entry, so the kind actually requested must match the grant.
func TestAuthorizer_OrgScope_KindGrantDoesNotCrossOver(t *testing.T) {
	tenants := fakeTenantResolver{bySlug: map[string]*iampb.Tenant{"acme": {Id: "tenant-acme"}}}
	perms := fakePermResolver{perms: map[string][]*iampb.Permission{
		"acc-1|tenant-acme": {{Resource: iampb.Resource_RESOURCE_WORKFLOW, Action: iampb.Action_ACTION_UPDATE}},
	}}
	a := &Authorizer{
		Tokens:  fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"tok": {AccountId: "acc-1"}}},
		Perms:   perms,
		Tenants: tenants,
	}
	if !a.CanAuthor(req("/ide/org/acme/workflow/oltp/", "tok")) {
		t.Fatal("caller with RESOURCE_WORKFLOW UPDATE must be able to author a workflow entry")
	}
	if a.CanAuthor(req("/ide/org/acme/provider/docker/", "tok")) {
		t.Fatal("caller with only RESOURCE_WORKFLOW UPDATE must NOT be able to author a provider entry")
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
	if a.CanAuthor(req("/ide/org/acme/provider/docker/", "tok")) {
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
	if !a.CanAuthor(req("/ide/org/acme/provider/docker/", "admin-tok")) {
		t.Fatal("platform admin must be able to author any org")
	}
}

func TestAuthorizer_FailsClosedOnMissingOrInvalidToken(t *testing.T) {
	a := &Authorizer{Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{}}}
	if a.CanAuthor(req("/ide/instance/provider/docker/", "")) {
		t.Fatal("missing bearer token must be rejected")
	}
	if a.CanAuthor(req("/ide/instance/provider/docker/", "garbage")) {
		t.Fatal("invalid bearer token must be rejected")
	}
}

func TestAuthorizer_FailsClosedOnUnparsableScope(t *testing.T) {
	a := &Authorizer{Tokens: fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"tok": {AccountId: "acc-1", IsAdmin: true}}}}
	if a.CanAuthor(req("/ide/", "tok")) {
		t.Fatal("an unparsable scope must be rejected even for an admin token")
	}
	if a.CanAuthor(req("/ide/instance/", "tok")) {
		t.Fatal("a scope missing its entry kind/slug must be rejected even for an admin token")
	}
}

func TestAuthorizer_FailsClosedOnUnknownOrgSlug(t *testing.T) {
	a := &Authorizer{
		Tokens:  fakeTokenVerifier{claims: map[string]*iampb.AccessClaims{"tok": {AccountId: "acc-1"}}},
		Perms:   fakePermResolver{},
		Tenants: fakeTenantResolver{bySlug: map[string]*iampb.Tenant{}},
	}
	if a.CanAuthor(req("/ide/org/nonexistent/provider/docker/", "tok")) {
		t.Fatal("an unknown org slug must be rejected")
	}
}
