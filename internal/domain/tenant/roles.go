package tenant

import (
	"context"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
)

// RoleIn is the token service's view of access: role by tenant id.
func (s *Service) RoleIn(ctx context.Context, actor auth.Actor, tenantID uuid.UUID) (slug, role string, err error) {
	t, err := s.repo.ByID(ctx, tenantID)
	if err != nil {
		return "", "", err
	}
	_, r, err := s.Access(ctx, actor, t.Slug)
	if err != nil {
		return "", "", err
	}
	return t.Slug, string(r), nil
}

// ByID reads a tenant without an access check — for rendering references
// the caller already holds (an invite's tenant).
func (s *Service) ByID(ctx context.Context, id uuid.UUID) (Tenant, error) {
	return s.repo.ByID(ctx, id)
}

// NamespaceOf is the Graphene namespace of a tenant (no access check —
// callers already resolved access).
func (s *Service) NamespaceOf(ctx context.Context, tenantID uuid.UUID) (string, error) {
	t, err := s.repo.ByID(ctx, tenantID)
	if err != nil {
		return "", err
	}
	return t.GrapheneNamespace, nil
}
