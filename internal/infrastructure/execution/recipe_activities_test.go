package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/provider"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/workflows"
)

// errQuotaInsufficientFixture is a fixture error fakeQuotaManager.Reserve
// returns to exercise ReserveQuotasActivity's error passthrough.
var errQuotaInsufficientFixture = errors.New("quota insufficient (fixture)")

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
	a := NewRecipeActivities(provider.Deps{}, nil)
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
	a := NewRecipeActivities(provider.Deps{}, nil)
	files := loadBundle(t, postgresHADir)
	delete(files, "cluster.yaml")

	out, err := a.CompileRecipeActivity(context.Background(), &workflows.CompileRecipeActivityInput{Bundle: files})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.True(t, out.Diagnostics.HasErrors())
	require.Nil(t, out.Plan)
}

func TestCompileRecipeActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, nil)
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
	a := NewRecipeActivities(provider.Deps{DockerExec: fake}, nil)

	out, err := a.ProvisionActivity(context.Background(), &workflows.ProvisionActivityInput{
		Groups:      groups(),
		ProviderRef: dockerRef(),
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.Machines["runner"], 2)
	require.Len(t, fake.ensured, 2)
}

// TestProvisionActivity_Docker_InjectsRuntimeParams reproduces the live stand
// scenario: a docker recipe whose provider.params carry none of the
// infra-authored runtime fields (image/server_addr/run_id/gateway_group). The
// activity must inject RunID/ServerAddr/GatewayGroup from its input (and the
// docker provider must default the image) so Provision no longer fails with
// "server_addr is required" / "run_id is required".
func TestProvisionActivity_Docker_InjectsRuntimeParams(t *testing.T) {
	fake := &fakeDockerExec{}
	a := NewRecipeActivities(provider.Deps{DockerExec: fake}, nil)

	out, err := a.ProvisionActivity(context.Background(), &workflows.ProvisionActivityInput{
		Groups:       groups(),
		ProviderRef:  &dslpb.ProviderRef{Name: "docker"}, // empty params, as a bundle's cluster.yaml produces
		RunID:        "run-xyz",
		ServerAddr:   "http://gateway:8080",
		GatewayGroup: "runner",
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.Machines["runner"], 2)
}

func TestProvisionActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, nil)
	out, err := a.ProvisionActivity(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, out)
}

func TestProvisionActivity_UnresolvableProvider_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, nil)

	out, err := a.ProvisionActivity(context.Background(), &workflows.ProvisionActivityInput{
		Groups:      groups(),
		ProviderRef: dockerRef(),
	})
	require.Error(t, err)
	require.Nil(t, out)
}

func TestTeardownActivity_Docker_CallsDestroy(t *testing.T) {
	fake := &fakeDockerExec{}
	a := NewRecipeActivities(provider.Deps{DockerExec: fake}, nil)

	err := a.TeardownActivity(context.Background(), &workflows.TeardownActivityInput{ProviderRef: dockerRef()})
	require.NoError(t, err)
	require.Contains(t, fake.removedNetworks, "stroppy-run-1")
}

func TestTeardownActivity_NilProviderRef_NoOp(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, nil)

	err := a.TeardownActivity(context.Background(), &workflows.TeardownActivityInput{ProviderRef: nil})
	require.NoError(t, err)
}

func TestTeardownActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, nil)
	err := a.TeardownActivity(context.Background(), nil)
	require.Error(t, err)
}

// fakeQuotaManager is a test double for QuotaManager — recipe_activities.go
// declares QuotaManager as an interface specifically so these activity tests
// never need a real *quotas.Manager (which needs a live postgres.DB).
type fakeQuotaManager struct {
	reserveErr error
	commitErr  error
	releaseErr error

	reserveCalls []reserveCall
	commitCalls  []runKey
	releaseCalls []runKey
}

type reserveCall struct {
	tenantID, runID, workflowID, provider string
	groups                                []*dslpb.MachineGroup
}

type runKey struct{ tenantID, runID string }

func (f *fakeQuotaManager) Reserve(_ context.Context, tenantID, runID, workflowID, provider string, groups []*dslpb.MachineGroup) error {
	f.reserveCalls = append(f.reserveCalls, reserveCall{tenantID, runID, workflowID, provider, groups})
	return f.reserveErr
}

func (f *fakeQuotaManager) Commit(_ context.Context, tenantID, runID string) error {
	f.commitCalls = append(f.commitCalls, runKey{tenantID, runID})
	return f.commitErr
}

func (f *fakeQuotaManager) Release(_ context.Context, tenantID, runID string) error {
	f.releaseCalls = append(f.releaseCalls, runKey{tenantID, runID})
	return f.releaseErr
}

func TestReserveQuotasActivity_CallsManagerReserveWithInput(t *testing.T) {
	fake := &fakeQuotaManager{}
	a := NewRecipeActivities(provider.Deps{}, fake)

	groups := []*dslpb.MachineGroup{{Name: "app", Count: 1, Cpu: 2, RamMb: 2048}}
	err := a.ReserveQuotasActivity(context.Background(), &workflows.ReserveQuotasActivityInput{
		TenantID: "tenant-1", RunID: "run-1", WorkflowID: "wf-1", Provider: "docker", Groups: groups,
	})
	require.NoError(t, err)
	require.Len(t, fake.reserveCalls, 1)
	require.Equal(t, "tenant-1", fake.reserveCalls[0].tenantID)
	require.Equal(t, "run-1", fake.reserveCalls[0].runID)
	require.Equal(t, "wf-1", fake.reserveCalls[0].workflowID)
	require.Equal(t, "docker", fake.reserveCalls[0].provider)
	require.Equal(t, groups, fake.reserveCalls[0].groups)
}

func TestReserveQuotasActivity_PropagatesManagerError(t *testing.T) {
	fake := &fakeQuotaManager{reserveErr: errQuotaInsufficientFixture}
	a := NewRecipeActivities(provider.Deps{}, fake)

	err := a.ReserveQuotasActivity(context.Background(), &workflows.ReserveQuotasActivityInput{
		TenantID: "tenant-1", RunID: "run-1",
	})
	require.ErrorIs(t, err, errQuotaInsufficientFixture)
}

func TestReserveQuotasActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, &fakeQuotaManager{})
	require.Error(t, a.ReserveQuotasActivity(context.Background(), nil))
}

func TestReserveQuotasActivity_NilQuotaManager_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, nil)
	err := a.ReserveQuotasActivity(context.Background(), &workflows.ReserveQuotasActivityInput{TenantID: "t", RunID: "r"})
	require.Error(t, err)
}

func TestCommitQuotasActivity_CallsManagerCommit(t *testing.T) {
	fake := &fakeQuotaManager{}
	a := NewRecipeActivities(provider.Deps{}, fake)

	err := a.CommitQuotasActivity(context.Background(), &workflows.CommitQuotasActivityInput{TenantID: "tenant-1", RunID: "run-1"})
	require.NoError(t, err)
	require.Equal(t, []runKey{{"tenant-1", "run-1"}}, fake.commitCalls)
}

func TestCommitQuotasActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, &fakeQuotaManager{})
	require.Error(t, a.CommitQuotasActivity(context.Background(), nil))
}

func TestReleaseQuotasActivity_CallsManagerRelease(t *testing.T) {
	fake := &fakeQuotaManager{}
	a := NewRecipeActivities(provider.Deps{}, fake)

	err := a.ReleaseQuotasActivity(context.Background(), &workflows.ReleaseQuotasActivityInput{TenantID: "tenant-1", RunID: "run-1"})
	require.NoError(t, err)
	require.Equal(t, []runKey{{"tenant-1", "run-1"}}, fake.releaseCalls)
}

func TestReleaseQuotasActivity_NilQuotaManager_IsANoOp(t *testing.T) {
	// Unlike Reserve/Commit, Release must tolerate a nil QuotaManager: the
	// teardown defer (runrecipe.go's releaseQuotas) calls it unconditionally,
	// including on paths where quota reservation was never reached.
	a := NewRecipeActivities(provider.Deps{}, nil)
	err := a.ReleaseQuotasActivity(context.Background(), &workflows.ReleaseQuotasActivityInput{TenantID: "t", RunID: "r"})
	require.NoError(t, err)
}

func TestReleaseQuotasActivity_NilInput_Errors(t *testing.T) {
	a := NewRecipeActivities(provider.Deps{}, &fakeQuotaManager{})
	require.Error(t, a.ReleaseQuotasActivity(context.Background(), nil))
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
