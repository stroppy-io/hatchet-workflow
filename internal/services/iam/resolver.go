package iam

import (
	"context"
	"errors"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

/*
	Resolver resolves a caller's effective permissions in a tenant: the live
	union of the permissions of every role granted by the caller's membership in
	that tenant (see iam/claims.proto). No membership -> no permissions. The
	is_admin short-circuit is the interceptor's job, not this resolver's.
*/

// MembershipReader fetches the caller's membership in one tenant.
type MembershipReader interface {
	GetByAccountTenant(ctx context.Context, accountID, tenantID string) (*iam.Membership, error)
}

// RoleReader fetches roles by id.
type RoleReader interface {
	GetMany(ctx context.Context, ids []string) ([]*iam.Role, error)
}

type Resolver struct {
	memberships MembershipReader
	roles       RoleReader
}

func NewResolver(memberships MembershipReader, roles RoleReader) *Resolver {
	return &Resolver{memberships: memberships, roles: roles}
}

var _ Authz = (*Resolver)(nil)

// EffectivePermissions returns the deduplicated union of permissions the account
// holds in the tenant. An account with no membership yields an empty set.
func (r *Resolver) EffectivePermissions(ctx context.Context, accountID, tenantID string) ([]*iam.Permission, error) {
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
