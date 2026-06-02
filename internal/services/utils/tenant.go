package utils

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// TenantReader fetches a Tenant by its canonical id. It is the shared read-only
// tenant port for services that only need to resolve/validate a tenant (confirm
// it exists, read its fields) rather than mutate it — so they depend on this one
// method instead of re-declaring it. Get returns derrors.ErrNotFound for an
// unknown id.
type TenantReader interface {
	Get(ctx context.Context, id string) (*iam.Tenant, error)
}
