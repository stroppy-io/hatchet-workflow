package identity

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// TenantGetter is the minimal stored-tenant read the TenantReader needs. The
// gormstore *TenantRepo satisfies it (its Get(ctx, id) (*iam.Tenant, error)).
type TenantGetter interface {
	Get(ctx context.Context, id string) (*iam.Tenant, error)
}

// TenantReader implements utils.TenantReader by delegating to a backing
// TenantGetter, resolving a tenant by its canonical id. Get propagates
// derrors.ErrNotFound for an unknown id (the backing repo's contract).
type TenantReader struct {
	tenants TenantGetter
}

var _ utils.TenantReader = (*TenantReader)(nil)

// NewTenantReader builds the reader over a stored-tenant getter (inject the
// gormstore *TenantRepo at wiring time).
func NewTenantReader(tenants TenantGetter) *TenantReader {
	return &TenantReader{tenants: tenants}
}

func (r *TenantReader) Get(ctx context.Context, id string) (*iam.Tenant, error) {
	return r.tenants.Get(ctx, id)
}
