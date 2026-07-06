package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/provider"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

// postgresHADir is the golden recipe bundle fixture reused across
// internal/dsl, internal/services/dsl and (here) recipe activity tests.
const postgresHADir = "../../../examples/dsl/postgres-ha"

// loadBundle walks dir and returns every regular file's contents keyed by
// its slash path relative to dir, matching RunRecipeInput.Bundle's shape —
// mirrors internal/services/dsl/service_test.go's own helper of the same
// name (a different package, so it is duplicated rather than shared).
func loadBundle(t *testing.T, dir string) map[string][]byte {
	t.Helper()

	files := map[string][]byte{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(p) //nolint:gosec // test-only, reading our own fixture tree.
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	if err != nil {
		t.Fatalf("loadBundle(%q): %v", dir, err)
	}
	return files
}

func TestCompileRecipeActivity_PostgresHA_NoErrorDiagnostics(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})
	files := loadBundle(t, postgresHADir)

	out, err := a.CompileRecipeActivity(context.Background(), &workflows.CompileRecipeActivityInput{Bundle: files})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.False(t, out.Diagnostics.HasErrors(), "expected no error diagnostics, got %+v", out.Diagnostics)
	require.NotNil(t, out.Plan)
	require.NotNil(t, out.Plan.GetProvider())
	require.Equal(t, "yandex", out.Plan.GetProvider().GetName())
}

func TestCompileRecipeActivity_BrokenBundle_ReturnsDiagnosticsNoGoError(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})
	files := loadBundle(t, postgresHADir)
	delete(files, "cluster.yaml")

	out, err := a.CompileRecipeActivity(context.Background(), &workflows.CompileRecipeActivityInput{Bundle: files})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.True(t, out.Diagnostics.HasErrors())
	require.Nil(t, out.Plan)
}

func TestCompileRecipeActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})
	out, err := a.CompileRecipeActivity(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, out)
}

func dockerRef() *dslpb.ProviderRef {
	return &dslpb.ProviderRef{
		Name: "docker",
		ParamsJson: `{"image":"stroppy-agent:latest","server_addr":"http://gateway:8080",` +
			`"binary_url":"http://gateway:8080/agent/binary","run_id":"run-1"}`,
	}
}

func groups() []*dslpb.MachineGroup {
	return []*dslpb.MachineGroup{{Name: "runner", Count: 2, Cpu: 2, RamMb: 4096}}
}

func TestProvisionActivity_Docker_ReturnsMachines(t *testing.T) {
	fake := &fakeDockerExec{}
	a := NewRecipeActivities(provider.Deps{DockerExec: fake})

	out, err := a.ProvisionActivity(context.Background(), &workflows.ProvisionActivityInput{
		Groups:      groups(),
		ProviderRef: dockerRef(),
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.Machines["runner"], 2)
	require.Len(t, fake.ensured, 2)
}

func TestProvisionActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})
	out, err := a.ProvisionActivity(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, out)
}

func TestProvisionActivity_UnresolvableProvider_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})

	out, err := a.ProvisionActivity(context.Background(), &workflows.ProvisionActivityInput{
		Groups:      groups(),
		ProviderRef: dockerRef(),
	})
	require.Error(t, err)
	require.Nil(t, out)
}

func TestTeardownActivity_Docker_CallsDestroy(t *testing.T) {
	fake := &fakeDockerExec{}
	a := NewRecipeActivities(provider.Deps{DockerExec: fake})

	err := a.TeardownActivity(context.Background(), &workflows.TeardownActivityInput{ProviderRef: dockerRef()})
	require.NoError(t, err)
	require.Contains(t, fake.removedNetworks, "stroppy-run-1")
}

func TestTeardownActivity_NilProviderRef_NoOp(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})

	err := a.TeardownActivity(context.Background(), &workflows.TeardownActivityInput{ProviderRef: nil})
	require.NoError(t, err)
}

func TestTeardownActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{})
	err := a.TeardownActivity(context.Background(), nil)
	require.Error(t, err)
}

// fakeDockerExec is a minimal test double satisfying provider's unexported
// dockerExec interface (Deps.DockerExec accepts any value with the right
// method set — see provider.Deps' own doc comment on Actor for why this
// compiles from outside the provider package).
type fakeDockerExec struct {
	ensured         []provider.ContainerSpec
	removedNetworks []string
}

func (f *fakeDockerExec) EnsureContainer(_ context.Context, spec provider.ContainerSpec) (provider.ContainerState, error) {
	f.ensured = append(f.ensured, spec)
	return provider.ContainerState{ID: "container-" + spec.Name, InternalIP: "10.0.0.1"}, nil
}

func (f *fakeDockerExec) RemoveContainers(_ context.Context, networkName string) error {
	f.removedNetworks = append(f.removedNetworks, networkName)
	return nil
}
