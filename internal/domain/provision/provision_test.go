package provision

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/deployment"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/settings"
)

func TestResolveVmSettings(t *testing.T) {
	yandexSettings := &settings.Settings{
		YandexCloud: &settings.YandexCloudSettings{
			VmSettings: &settings.YandexCloudSettings_VmSettings{
				BaseImageId:     "yc-image-123",
				EnablePublicIps: true,
				VmUser: &deployment.VmUser{
					Name: "yc-user",
				},
			},
		},
		Docker: &settings.DockerSettings{
			EdgeWorkerImage: "stroppy-edge:latest",
		},
	}

	tests := []struct {
		name         string
		target       deployment.Target
		wantUser     *deployment.VmUser
		wantImageId  string
		wantPublicIp bool
		wantErr      bool
	}{
		{
			name:         "yandex cloud returns yandex vm settings",
			target:       deployment.Target_TARGET_YANDEX_CLOUD,
			wantUser:     &deployment.VmUser{Name: "yc-user"},
			wantImageId:  "yc-image-123",
			wantPublicIp: true,
		},
		{
			name:         "docker returns edge worker image and no user",
			target:       deployment.Target_TARGET_DOCKER,
			wantUser:     nil,
			wantImageId:  "stroppy-edge:latest",
			wantPublicIp: false,
		},
		{
			name:    "unknown target returns error",
			target:  deployment.Target(999),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vmUser, baseImageId, hasPublicIp, err := resolveVmSettings(tt.target, yandexSettings)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.wantUser == nil {
				assert.Nil(t, vmUser)
			} else {
				require.NotNil(t, vmUser)
				assert.Equal(t, tt.wantUser.GetName(), vmUser.GetName())
			}
			assert.Equal(t, tt.wantImageId, baseImageId)
			assert.Equal(t, tt.wantPublicIp, hasPublicIp)
		})
	}
}
