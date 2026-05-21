// Package authz enforces per-tenant RBAC. Because every ui request carries an
// inline tenant_id (A, H52), the role check cannot live in the auth interceptor
// (which sees no request body) — services call Require with the request's
// tenant_id and the RPC's minimum role. See features/tenancy/rbac.feature.
package authz

import (
	"context"
	"errors"

	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Authz resolves and enforces tenant membership roles.
type Authz struct {
	*tracing.Entity
	memberRepo *repository.ProtoRepository[
		models.TenantMemberAlias,
		models.TenantMemberColumnAlias,
		*models.TenantMemberScanner,
		*models.TenantMember,
	]
}

// New builds an Authz over the given DB executor.
func New(logger *xlog.Logger, executor exec.DB) *Authz {
	return &Authz{
		Entity: tracing.NewEntity(logger.AppendName("Authz")),
		memberRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(models.TenantMembers.Table, executor),
			models.TenantMemberConverter,
		),
	}
}

var (
	errUnauthenticated = status.Error(codes.Unauthenticated, "authentication required")
	errForbidden       = status.Error(codes.PermissionDenied, "insufficient role for tenant")
	errWrongTenant     = status.Error(codes.PermissionDenied, "token not scoped to this tenant")
	errNotMember       = status.Error(codes.PermissionDenied, "not a member of tenant")
	errAgentPrincipal  = status.Error(codes.PermissionDenied, "principal not allowed for tenant operations")
)

// Require verifies the caller may act in tenantID with at least minRole. Platform
// admins (Account.is_admin) bypass tenant roles. An API token must be scoped to
// tenantID and carry >= minRole. An account principal's role is resolved from
// its TenantMember row.
func (a *Authz) Require(
	ctx context.Context,
	c *caller.Caller,
	tenantID *models.TenantId,
	minRole models.TenantMember_Role,
) error {
	if c == nil {
		return errUnauthenticated
	}
	if c.IsAdmin {
		return nil
	}
	switch c.Kind {
	case caller.PrincipalApiToken:
		if c.TenantID.GetValue() != tenantID.GetValue() {
			return errWrongTenant
		}
		if c.Role < minRole {
			return errForbidden
		}
		return nil
	case caller.PrincipalAccount:
		role, err := a.roleInTenant(ctx, c.AccountID, tenantID)
		if err != nil {
			return err
		}
		if role < minRole {
			return errForbidden
		}
		return nil
	default:
		return errAgentPrincipal
	}
}

// roleInTenant resolves the account's membership role in the tenant; a missing
// membership maps to PermissionDenied (isolation: non-members are denied).
func (a *Authz) roleInTenant(
	ctx context.Context,
	accountID *models.AccountId,
	tenantID *models.TenantId,
) (models.TenantMember_Role, error) {
	member, err := a.memberRepo.QueryRow(
		ctx,
		models.TenantMembers.SelectAll().Where(
			models.TenantMembers.TenantId.Eq(tenantID.GetValue()),
			models.TenantMembers.AccountId.Eq(accountID.GetValue()),
			models.TenantMembers.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.TenantMember_ROLE_UNSPECIFIED, errNotMember
		}
		return models.TenantMember_ROLE_UNSPECIFIED, status.Errorf(codes.Internal, "resolve tenant role: %v", err)
	}
	return member.GetRole(), nil
}
