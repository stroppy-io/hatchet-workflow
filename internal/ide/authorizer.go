package ide

import (
	"context"
	"net/http"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/gateway"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// TokenVerifier validates a bearer access token — the same shape
// internal/services/iam.TokenVerifier already declares (*iamsvc.
// JWTTokenService/CompositeTokenVerifier satisfy this structurally; not
// imported directly to avoid gateway/ide importing services/iam's full
// dependency surface just for one method signature).
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*iampb.AccessClaims, error)
}

// PermissionResolver resolves a caller's effective permissions in one
// tenant — the same shape internal/services/iam.Resolver already
// implements (Authz interface there).
type PermissionResolver interface {
	EffectivePermissions(ctx context.Context, accountID, tenantID string) ([]*iampb.Permission, error)
}

// TenantResolver maps a URL-facing tenant slug to its internal id — the
// same lookup internal/services/iam.TenantRepo.GetBySlug already performs;
// declared narrowly here so a fake is trivial in tests.
type TenantResolver interface {
	GetBySlug(ctx context.Context, slug string) (*iampb.Tenant, error)
}

// Authorizer implements gateway.IdeAuthorizer against SP-B's real RBAC:
//   - ScopeInstance requires AccessClaims.IsAdmin (platform admin) — the
//     instance repo is admin_only everywhere else in this codebase
//     (catalog.Service's CreateInstanceEntry/UpdateInstanceEntry docs), so
//     the IDE route holds the same line: only a platform admin may author
//     the instance repo.
//   - ScopeOrg requires the caller to hold RESOURCE_PROVIDER or
//     RESOURCE_WORKFLOW ACTION_UPDATE/ACTION_MANAGE in the SPECIFIC tenant
//     the URL slug resolves to (never a different org's tenant — see
//     CanAuthor's doc) — the same permission pair catalog.Service's
//     UpdateOrgProvider/UpdateOrgWorkflow RPCs are gated on today (all_of
//     RESOURCE_PROVIDER/RESOURCE_WORKFLOW, per service.go's file doc), so
//     "can author this org's catalog via the API" and "can author it via
//     the IDE" resolve to the identical permission check — no parallel
//     authz path.
//   - IsAdmin short-circuits both cases (mirrors internal/services/iam/
//     auth.go's authorize: "is_admin short-circuits").
//   - Any resolution failure (bad/missing token, unknown slug, resolver
//     error) is a false, never a panic — CanAuthor's documented contract.
type Authorizer struct {
	Tokens  TokenVerifier
	Perms   PermissionResolver
	Tenants TenantResolver
}

var _ gateway.IdeAuthorizer = (*Authorizer)(nil)

// CanAuthor implements gateway.IdeAuthorizer.
func (a *Authorizer) CanAuthor(r *http.Request) bool {
	if a == nil || a.Tokens == nil {
		return false
	}
	scope, err := ParseScope(r.URL.Path)
	if err != nil {
		return false
	}
	claims, ok := a.verifiedClaims(r)
	if !ok {
		return false
	}
	if claims.GetIsAdmin() {
		return true
	}
	switch scope.Kind {
	case ScopeInstance:
		// Non-admins never author the instance repo — no permission grant
		// substitutes for platform admin here, matching every other
		// LEVEL_INSTANCE write path in this codebase.
		return false
	case ScopeOrg:
		return a.canAuthorOrg(r.Context(), claims.GetAccountId(), scope.OrgSlug)
	default:
		return false
	}
}

func (a *Authorizer) canAuthorOrg(ctx context.Context, accountID, orgSlug string) bool {
	if a.Tenants == nil || a.Perms == nil {
		return false
	}
	tenant, err := a.Tenants.GetBySlug(ctx, orgSlug)
	if err != nil || tenant.GetId() == "" {
		return false
	}
	perms, err := a.Perms.EffectivePermissions(ctx, accountID, tenant.GetId())
	if err != nil {
		return false
	}
	return hasAuthoringPermission(perms)
}

// hasAuthoringPermission reports whether perms grants write access to the
// org catalog — RESOURCE_PROVIDER or RESOURCE_WORKFLOW at ACTION_UPDATE or
// ACTION_MANAGE, the same resource pair catalog.Service's
// UpdateOrgProvider/UpdateOrgWorkflow RPCs require (see service.go's
// package doc: "so all_of can gate RESOURCE_PROVIDER separately from
// RESOURCE_WORKFLOW"). Either resource alone is sufficient here — the IDE
// is a single shared workspace covering both providers/ and workflows/, so
// requiring both would deny a caller who e.g. only has workflow-authoring
// rights from opening the IDE at all, even to edit only workflows/.
func hasAuthoringPermission(perms []*iampb.Permission) bool {
	for _, p := range perms {
		switch p.GetResource() {
		case iampb.Resource_RESOURCE_PROVIDER, iampb.Resource_RESOURCE_WORKFLOW:
			switch p.GetAction() {
			case iampb.Action_ACTION_UPDATE, iampb.Action_ACTION_MANAGE, iampb.Action_ACTION_CREATE:
				return true
			}
		}
	}
	return false
}

// verifiedClaims extracts and verifies the bearer token from r's
// Authorization header. Unlike internal/infrastructure/identity.bearerToken
// (which reads gRPC incoming metadata), this reads a plain net/http
// request — the gateway's /ide/* route is HTTP/1.1, not connect/gRPC, and
// runs before any connect-rpc interceptor ever sees the request (see
// gateway.go's serveHTTP dispatch), so claims are never pre-stashed in the
// context the way iamsvc.ClaimsFromContext expects.
func (a *Authorizer) verifiedClaims(r *http.Request) (*iampb.AccessClaims, bool) {
	const prefix = "bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return nil, false
	}
	token := strings.TrimSpace(h[len(prefix):])
	if token == "" {
		return nil, false
	}
	claims, err := a.Tokens.Verify(r.Context(), token)
	if err != nil || claims == nil {
		return nil, false
	}
	return claims, true
}
