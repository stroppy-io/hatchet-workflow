package provider

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

func TestNewProviderForRef_Docker_ReturnsDockerBackedProvider(t *testing.T) {
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "docker"}, RunContext{}, Deps{DockerExec: &fakeDockerExec{}})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewProviderForRef_Docker_NilDockerExec_Errors(t *testing.T) {
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "docker"}, RunContext{}, Deps{})
	require.Error(t, err)
	require.Nil(t, p)
}

func TestNewProviderForRef_Yandex_ReturnsTerraformBackedProvider(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	deps := Deps{
		Actor: &fakeTfActor{},
		ModuleDir: func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
			require.Equal(t, "yandex", name)
			return "yandex", tfFiles, true
		},
	}

	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"}, RunContext{}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewProviderForRef_UnknownProvider_ErrorsWithName(t *testing.T) {
	deps := Deps{
		ModuleDir: func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
			return "", nil, false
		},
	}

	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "nope"}, RunContext{}, deps)
	require.Error(t, err)
	require.Nil(t, p)
	require.Contains(t, err.Error(), "nope")
}

func TestNewProviderForRef_NonDocker_NilModuleDir_Errors(t *testing.T) {
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"}, RunContext{}, Deps{})
	require.Error(t, err)
	require.Nil(t, p)
}

func TestNewProviderForRef_Yandex_ResolvesEnvViaEnvFnWithTenantID(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	var gotTenant string
	deps := Deps{
		Actor: &fakeTfActor{},
		ModuleDir: func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
			return "yandex", tfFiles, true
		},
		EnvFn: func(_ context.Context, tenantID string) (map[string]string, error) {
			gotTenant = tenantID
			return map[string]string{"YC_TOKEN": "resolved-secret"}, nil
		},
	}

	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"},
		RunContext{TenantID: "tenant-1", RunID: "run-1"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, "tenant-1", gotTenant)
}

func TestNewProviderForRef_Yandex_EnvFnError_Propagates(t *testing.T) {
	deps := Deps{
		Actor:     &fakeTfActor{},
		ModuleDir: func(tenantID, runID, name string) (string, []terraform.TfFile, bool) { return "yandex", nil, true },
		EnvFn: func(context.Context, string) (map[string]string, error) {
			return nil, fmt.Errorf("no deploy credentials configured for tenant")
		},
	}
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"},
		RunContext{TenantID: "tenant-1"}, deps)
	require.Error(t, err)
	require.Nil(t, p)
}

func TestNewProviderForRef_Docker_NeverCallsEnvFn(t *testing.T) {
	called := false
	deps := Deps{
		DockerExec: &fakeDockerExec{},
		EnvFn: func(context.Context, string) (map[string]string, error) {
			called = true
			return nil, nil
		},
	}
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "docker"},
		RunContext{TenantID: "tenant-1"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.False(t, called, "docker builtin needs no terraform credentials")
}

func TestNewProviderForRef_Docker_NeverCallsLogSinkFn(t *testing.T) {
	called := false
	deps := Deps{
		DockerExec: &fakeDockerExec{},
		LogSinkFn: func(context.Context, string) (io.Writer, io.Writer) {
			called = true
			return nil, nil
		},
	}
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "docker"},
		RunContext{TenantID: "tenant-1"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.False(t, called, "docker builtin needs no per-run log sink")
}

func TestNewProviderForRef_Yandex_ResolvesLogSinkViaRunID(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	var gotRunID string
	deps := Deps{
		Actor: &fakeTfActor{},
		ModuleDir: func(tenantID, runID, name string) (string, []terraform.TfFile, bool) {
			return "yandex", tfFiles, true
		},
		LogSinkFn: func(_ context.Context, runID string) (io.Writer, io.Writer) {
			gotRunID = runID
			return nil, nil
		},
	}
	p, err := NewProviderForRef(context.Background(), &dslpb.ProviderRef{Name: "yandex"},
		RunContext{TenantID: "tenant-1", RunID: "run-1"}, deps)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, "run-1", gotRunID)
}
