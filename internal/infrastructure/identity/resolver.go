package identity

import (
	"context"
	"errors"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// MembershipReader fetches the caller's membership in one tenant. The gormstore
// *MembershipRepo satisfies it. GetByAccountTenant returns derrors.ErrNotFound
// when the account has no membership in the tenant.
type MembershipReader interface {
	GetByAccountTenant(ctx context.Context, accountID, tenantID string) (*iam.Membership, error)
}

// RoleReader fetches roles by id. The gormstore *RoleRepo satisfies it.
type RoleReader interface {
	GetMany(ctx context.Context, ids []string) ([]*iam.Role, error)
}

// PermissionResolver resolves a caller's effective permissions in one tenant:
// the deduplicated union of the permissions of every role bound by the caller's
// membership in that tenant. It implements both iamsvc.Authz and
// iamsvc.PermissionResolver (the same EffectivePermissions signature).
//
// No membership -> empty set. The is_admin platform short-circuit is the gate's
// job, not this resolver's.
type PermissionResolver struct {
	memberships MembershipReader
	roles       RoleReader
}

var (
	_ iamsvc.Authz              = (*PermissionResolver)(nil)
	_ iamsvc.PermissionResolver = (*PermissionResolver)(nil)
)

// NewPermissionResolver builds the resolver over stored membership/role readers
// (inject the gormstore *MembershipRepo and *RoleRepo at wiring time).
func NewPermissionResolver(memberships MembershipReader, roles RoleReader) *PermissionResolver {
	return &PermissionResolver{memberships: memberships, roles: roles}
}

// EffectivePermissions returns the deduplicated union of permissions the account
// holds in the tenant. An account with no membership yields an empty set.
func (r *PermissionResolver) EffectivePermissions(ctx context.Context, accountID, tenantID string) ([]*iam.Permission, error) {
	m, err := r.memberships.GetByAccountTenant(ctx, accountID, tenantID)
	if errors.Is(err, derrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	roles, err := r.roles.GetMany(ctx, m.GetRoleIds())
	if err != nil {
		return nil, err
	}
	return unionPermissions(roles), nil
}

func unionPermissions(roles []*iam.Role) []*iam.Permission {
	seen := make(map[[2]int32]bool)
	var out []*iam.Permission
	for _, role := range roles {
		for _, p := range role.GetPermissions() {
			key := [2]int32{int32(p.GetResource()), int32(p.GetAction())}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, p)
		}
	}
	return out
}
