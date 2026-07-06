package provider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

func TestNewProviderForRef_Docker_ReturnsDockerBackedProvider(t *testing.T) {
	p, err := NewProviderForRef(&dslpb.ProviderRef{Name: "docker"}, Deps{DockerExec: &fakeDockerExec{}})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewProviderForRef_Docker_NilDockerExec_Errors(t *testing.T) {
	p, err := NewProviderForRef(&dslpb.ProviderRef{Name: "docker"}, Deps{})
	require.Error(t, err)
	require.Nil(t, p)
}

func TestNewProviderForRef_Yandex_ReturnsTerraformBackedProvider(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	deps := Deps{
		Actor: &fakeTfActor{},
		ModuleDir: func(name string) (string, []terraform.TfFile, bool) {
			require.Equal(t, "yandex", name)
			return "yandex", tfFiles, true
		},
	}

	p, err := NewProviderForRef(&dslpb.ProviderRef{Name: "yandex"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewProviderForRef_UnknownProvider_ErrorsWithName(t *testing.T) {
	deps := Deps{
		ModuleDir: func(string) (string, []terraform.TfFile, bool) {
			return "", nil, false
		},
	}

	p, err := NewProviderForRef(&dslpb.ProviderRef{Name: "nope"}, deps)
	require.Error(t, err)
	require.Nil(t, p)
	require.Contains(t, err.Error(), "nope")
}

func TestNewProviderForRef_NonDocker_NilModuleDir_Errors(t *testing.T) {
	p, err := NewProviderForRef(&dslpb.ProviderRef{Name: "yandex"}, Deps{})
	require.Error(t, err)
	require.Nil(t, p)
}
