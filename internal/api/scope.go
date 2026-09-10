package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
)

// tenantOf resolves the caller and the tenant of a /t/{slug} path. Role
// checks happen in the services; here only "member at all".
func (h *Handler) tenantOf(ctx context.Context, slug string) (auth.Actor, tenant.Tenant, error) {
	a, err := actor(ctx)
	if err != nil {
		return auth.Actor{}, tenant.Tenant{}, err
	}
	t, err := h.deps.Tenants.Get(ctx, a, slug)
	if err != nil {
		return auth.Actor{}, tenant.Tenant{}, err
	}
	return a, t, nil
}
