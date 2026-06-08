package tenant_settings

import (
	"testing"

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
