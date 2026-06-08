package tenant_settings

import (
	"testing"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestApplyProviderSettingsPreservesDefaultProvider(t *testing.T) {
	rec := &models.TenantSettingsRecord{
		DefaultProvider: deployment.Provider_PROVIDER_DOCKER,
	}
	yandex := &deployment.Yandex_Settings{CloudId: "cloud-1"}

	applyProviderSettings(rec, &deployment.ProviderSettings{
		Settings: &deployment.ProviderSettings_Yandex{Yandex: yandex},
	})

	if rec.GetDefaultProvider() != deployment.Provider_PROVIDER_DOCKER {
		t.Fatalf("default provider changed to %s", rec.GetDefaultProvider())
	}
	if rec.GetYandexSettings() != yandex {
		t.Fatal("yandex provider settings were not stored")
	}
}

func TestApplyDockerProviderSettingsPreservesDefaultProvider(t *testing.T) {
	rec := &models.TenantSettingsRecord{
		DefaultProvider: deployment.Provider_PROVIDER_YANDEX,
	}

	applyProviderSettings(rec, &deployment.ProviderSettings{
		Settings: &deployment.ProviderSettings_Docker{
			Docker: &deployment.Docker_Settings{},
		},
	})

	if rec.GetDefaultProvider() != deployment.Provider_PROVIDER_YANDEX {
		t.Fatalf("default provider changed to %s", rec.GetDefaultProvider())
	}
}

func TestMergeEntitySeedsValidSettingsName(t *testing.T) {
	svc := &TenantSettingsService{}

	entity := svc.mergeEntity(nil, "tenant-1", "account-1")

	if entity.GetName() != tenantSettingsEntityName {
		t.Fatalf("name = %q, want %q", entity.GetName(), tenantSettingsEntityName)
	}
	if err := entity.ValidateAll(); err != nil {
		t.Fatalf("entity should validate: %v", err)
	}
}

func TestMergeEntityBackfillsEmptySettingsName(t *testing.T) {
	svc := &TenantSettingsService{}
	prior := &commonpb.Entity{
		Id:       "settings-1",
		TenantId: "tenant-1",
	}

	entity := svc.mergeEntity(prior, "tenant-1", "account-1")

	if entity.GetName() != tenantSettingsEntityName {
		t.Fatalf("name = %q, want %q", entity.GetName(), tenantSettingsEntityName)
	}
	if err := entity.ValidateAll(); err != nil {
		t.Fatalf("entity should validate: %v", err)
	}
}
