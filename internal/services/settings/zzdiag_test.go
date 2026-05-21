//go:build integration

package settings_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestDiagUpsert(t *testing.T) {
	f := newFixture(t)
	ctx := f.ownerCtx()

	c1, err := f.svc.SetSettingsItem(ctx, &uipb.SetSettingsItemRequest{
		TenantId: f.tenantID,
		Part:     models.SettingsItem_PART_YANDEX_CLOUD,
		Key:      models.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
		Value:    &models.SettingsItem_Value{Value: &models.SettingsItem_Value_StringValue{StringValue: "tok-1"}},
	})
	require.NoError(t, err)
	t.Logf("created id=%s part=%v key=%v", c1.GetId().GetValue(), c1.GetPart(), c1.GetKey())

	list, err := f.svc.ListSettingsItems(ctx, &uipb.ListSettingsItemsRequest{TenantId: f.tenantID})
	require.NoError(t, err)
	for _, it := range list.GetSettingsItems() {
		t.Logf("listed id=%s part=%v(%s) key=%v(%s)", it.GetId().GetValue(), it.GetPart(), it.GetPart().String(), it.GetKey(), it.GetKey().String())
	}
}
