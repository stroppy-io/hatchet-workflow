// Package caller carries the authenticated principal (account, API token, or
// agent) through the request context. The auth middleware populates it; handlers
// and RBAC guards read it. See features/tenancy/{auth,rbac}.feature and
// models/{account,tenant,apitoken,agent}.proto for the contract.
package caller

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// PrincipalKind identifies which of the three auth principals a request carries
// (A, features/tenancy/auth.feature).
type PrincipalKind int

const (
	// PrincipalUnknown is the zero value (anonymous / unset).
	PrincipalUnknown PrincipalKind = iota
	// PrincipalAccount is a human account authenticated by a JWT access token.
	PrincipalAccount
	// PrincipalApiToken is a tenant-scoped CI/SDK Bearer token (role capped <= ADMIN).
	PrincipalApiToken
	// PrincipalAgent is a machine principal (per-machine JWT from cloud-init).
	PrincipalAgent
)

// Caller is the authenticated principal for a request. Which fields are set
// depends on Kind: account -> AccountID(+IsAdmin); API token -> TenantID+Role;
// agent -> AgentID(+TenantID). Per-tenant role for an account principal is NOT
// here — it is resolved per-request from the inline tenant_id by services/authz,
// since an account may belong to several tenants.
type Caller struct {
	// Kind selects which principal this is.
	Kind PrincipalKind
	// AccountID is set for PrincipalAccount.
	AccountID *models.AccountId
	// IsAdmin grants platform-level authority independent of tenant role
	// (Account.is_admin). Set for PrincipalAccount.
	IsAdmin bool
	// TenantID is the bound tenant for PrincipalApiToken / PrincipalAgent.
	TenantID *models.TenantId
	// Role is the effective role for PrincipalApiToken (capped <= ADMIN).
	Role models.TenantMember_Role
	// AgentID is set for PrincipalAgent.
	AgentID *models.AgentId
}

// AtLeast reports whether the caller's carried role meets the minimum. Platform
// admins satisfy any role. Role values are ordered VIEWER < ADMIN < OWNER. Only
// meaningful for principals that carry a role (API token); for an account
// principal use services/authz to resolve the per-tenant role first.
func (c *Caller) AtLeast(min models.TenantMember_Role) bool {
	if c == nil {
		return false
	}
	if c.IsAdmin {
		return true
	}
	return c.Role >= min
}

type ctxKey struct{}

// NewContext returns a copy of ctx carrying the caller.
func NewContext(ctx context.Context, c *Caller) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the caller stored in ctx. ok is false for anonymous requests.
func FromContext(ctx context.Context) (c *Caller, ok bool) {
	c, ok = ctx.Value(ctxKey{}).(*Caller)
	return c, ok && c != nil
}
