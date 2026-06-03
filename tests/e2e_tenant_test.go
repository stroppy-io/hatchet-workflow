//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
)

// TestE2ETenant exercises the multi-tenant surface: list my tenants, read
// tenant settings, and read quotas.
func TestE2ETenant(t *testing.T) {
	e := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ListMyTenants — admin must own at least the seeded "default" tenant.
	tr, err := e.iam.ListMyTenants(ctx, &api.ListMyTenantsRequest{})
	if err != nil {
		t.Fatalf("ListMyTenants: %v", err)
	}
	if len(tr.GetTenants()) == 0 {
		t.Fatal("ListMyTenants: admin owns no tenants")
	}
	for _, tn := range tr.GetTenants() {
		t.Logf("tenant id=%s name=%s", tn.GetId(), tn.GetName())
	}

	// Tenant settings must be readable for the resolved tenant.
	gs, err := e.settings.GetTenantSettings(ctx, &api.GetTenantSettingsRequest{
		TenantId: e.tenantID,
	})
	if err != nil {
		t.Fatalf("GetTenantSettings(%s): %v", e.tenantID, err)
	}
	if gs.GetSettings() == nil {
		t.Fatal("GetTenantSettings: nil settings")
	}

	// Quotas must be readable.
	if _, err := e.quota.ListQuotas(ctx, &api.ListQuotasRequest{
		TenantId: e.tenantID,
	}); err != nil {
		t.Fatalf("ListQuotas(%s): %v", e.tenantID, err)
	}
	t.Logf("quotas read OK for tenant %s", e.tenantID)
}
