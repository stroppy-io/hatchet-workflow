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
	// Sessions verifies the httpOnly IDE session cookie the browser-auth
	// handshake sets (TicketExchanger, ticket.go) — the second credential
	// shape CanAuthor accepts, alongside the Authorization header. nil
	// disables cookie-based auth entirely (CanAuthor then behaves exactly
	// as it did before the browser-auth handshake existed: header only),
	// which keeps every existing curl/API/test caller unaffected.
	Sessions SessionVerifier
}

var _ gateway.IdeAuthorizer = (*Authorizer)(nil)

// CanAuthor implements gateway.IdeAuthorizer. Two independent credential
// shapes are accepted, checked in this order:
//
//  1. The stroppy_ide_session cookie (if Sessions is configured and the
//     cookie is present): proof that THIS exact scope was already
//     authorized once, at ticket-mint time, using the caller's Bearer
//     token (TicketIssuer.Issue runs this very method against the target
//     scope before ever minting a ticket — see its doc). The cookie is
//     scope-bound (its own "scope" claim, plus the Path attribute the
//     browser enforces) so it can never be replayed against a different
//     scope; verifiedSession checks the claim explicitly rather than
//     trusting Path alone, since Path scoping is a browser-side courtesy,
//     not a server-side guarantee against a forged Cookie header. A valid
//     cookie short-circuits straight to true — no repeated permission
//     lookup on every asset/websocket-frame request for the lifetime of
//     the IDE session, by design (see ticket.go's package doc for the
//     trust boundary this establishes).
//  2. The Authorization header, exactly as before this handshake existed:
//     full claims verification + IsAdmin/RESOURCE_PROVIDER/RESOURCE_WORKFLOW
//     permission resolution. This is what curl/the API/every existing test
//     still uses, and what TicketIssuer.Issue itself uses to authorize the
//     ticket mint in the first place.
func (a *Authorizer) CanAuthor(r *http.Request) bool {
	if a == nil {
		return false
	}
	scope, err := ParseScope(r.URL.Path)
	if err != nil {
		return false
	}
	if a.Sessions != nil {
		if a.verifiedSession(r, scope.Key()) {
			return true
		}
	}
	if a.Tokens == nil {
		return false
	}
	if scope.EntryKind == EntryKindRecipe && scope.Kind == ScopeInstance {
		// A recipe is always tenant-owned (see recipe.recipeBundleIdentity's
		// doc — there is no LEVEL_INSTANCE recipe): reject BEFORE the
		// IsAdmin short-circuit below, so even a platform admin cannot open
		// an instance-level recipe scope — unlike provider/workflow, there
		// is no legitimate "instance recipe repo" for any credential to
		// open, admin or not.
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
		return a.canAuthorOrg(r.Context(), claims.GetAccountId(), scope.OrgSlug, scope.EntryKind)
	default:
		return false
	}
}

// canAuthorOrg checks the caller's permission against the resource matching
// entryKind SPECIFICALLY (RESOURCE_PROVIDER for EntryKindProvider,
// RESOURCE_WORKFLOW for EntryKindWorkflow) — tighter than the pre-redesign
// version, which granted access to the whole shared org worktree (covering
// both providers/ and workflows/) to a caller holding EITHER resource's
// grant. Now that a Scope names one specific entry repo, a caller who only
// holds RESOURCE_WORKFLOW's grant must not be able to open a
// RESOURCE_PROVIDER entry's repo (and vice versa) — this is what "IDE
// authoring permission == the identical RBAC UpdateOrgProvider/
// UpdateOrgWorkflow enforce" now means at the per-entry granularity those
// RPCs already had.
func (a *Authorizer) canAuthorOrg(ctx context.Context, accountID, orgSlug, entryKind string) bool {
	if a.Tenants == nil || a.Perms == nil {
		return false
	}
	resource, ok := resourceForEntryKind(entryKind)
	if !ok {
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
	return hasAuthoringPermission(perms, resource)
}

// resourceForEntryKind maps a Scope.EntryKind string to the iampb.Resource
// catalog.Service's UpdateOrgProvider/UpdateOrgWorkflow RPCs are gated on
// for that same kind — the single place this string<->enum mapping is made,
// so the IDE and the catalog RPC surface can never silently drift apart.
func resourceForEntryKind(entryKind string) (iampb.Resource, bool) {
	switch entryKind {
	case EntryKindProvider:
		return iampb.Resource_RESOURCE_PROVIDER, true
	case EntryKindWorkflow:
		return iampb.Resource_RESOURCE_WORKFLOW, true
	case EntryKindRecipe:
		return iampb.Resource_RESOURCE_RECIPE, true
	default:
		return iampb.Resource_RESOURCE_UNSPECIFIED, false
	}
}

// hasAuthoringPermission reports whether perms grants write access to
// resource specifically, at ACTION_UPDATE/ACTION_MANAGE/ACTION_CREATE — the
// same actions catalog.Service's UpdateOrgProvider/UpdateOrgWorkflow RPCs
// require.
func hasAuthoringPermission(perms []*iampb.Permission, resource iampb.Resource) bool {
	for _, p := range perms {
		if p.GetResource() != resource {
			continue
		}
		switch p.GetAction() {
		case iampb.Action_ACTION_UPDATE, iampb.Action_ACTION_MANAGE, iampb.Action_ACTION_CREATE:
			return true
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

// verifiedSession reports whether r carries a valid stroppy_ide_session
// cookie whose scope claim matches wantScope exactly. A cookie present but
// invalid (bad signature, expired, wrong purpose) or scoped to a DIFFERENT
// key (a different org, a different entry, instance vs. org) is rejected,
// never treated as "no cookie" silently upgraded to something else — the
// caller falls through to the Authorization-header path, which will also
// fail closed if that is likewise absent/invalid.
func (a *Authorizer) verifiedSession(r *http.Request, wantScope string) bool {
	c, err := r.Cookie(ideSessionCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	accountID, scope, err := a.Sessions.VerifySession(r.Context(), c.Value)
	if err != nil || accountID == "" || scope != wantScope {
		return false
	}
	return true
}
