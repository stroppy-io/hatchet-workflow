package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

var assertErr = errors.New("boom")

// fakeTfActor is a test double for tfActor: it records the *WorkdirWithParams
// each call was built with (all we can observe about it are the fields
// exposed by exported accessors — WorkdirPath and StateFilePresent — since
// tfFiles/varFile/env are unexported and the terraform package intentionally
// exposes no accessor for them) and returns canned results.
type fakeTfActor struct {
	applyWorkdir *terraform.WorkdirWithParams
	applyOutput  terraform.TfOutput
	applyErr     error

	destroyWd   terraform.WdId
	destroyOpts []terraform.Option
	destroyErr  error
}

func (f *fakeTfActor) ApplyTerraform(_ context.Context, w *terraform.WorkdirWithParams) (terraform.TfOutput, error) {
	f.applyWorkdir = w
	return f.applyOutput, f.applyErr
}

func (f *fakeTfActor) DestroyExisting(_ context.Context, wd terraform.WdId, opts ...terraform.Option) error {
	f.destroyWd = wd
	f.destroyOpts = opts
	return f.destroyErr
}

func TestTerraformActorExec_Apply_BuildsWorkdirAndReturnsOutputs(t *testing.T) {
	fake := &fakeTfActor{
		applyOutput: terraform.TfOutput{"stroppy_machines": []byte(`[]`)},
	}
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	env := map[string]string{"YC_TOKEN": "secret"}
	adapter := NewTerraformActorExec(fake, tfFiles, env)

	varsJSON := []byte(`{"stroppy_nodes":[]}`)
	out, err := adapter.Apply(context.Background(), "run-1", varsJSON)
	require.NoError(t, err)

	// Output round-trips from the fake's TfOutput to the plain map the
	// terraformExec interface promises.
	require.Equal(t, map[string][]byte{"stroppy_machines": []byte(`[]`)}, out)

	// The workdir passed to ApplyTerraform is non-nil and its WdId (the only
	// field observable via an exported accessor, WorkdirPath) was derived
	// from dir.
	require.NotNil(t, fake.applyWorkdir)
	require.Equal(t, terraform.NewWorkdirWithParams(terraform.NewWdId("run-1")).WorkdirPath(), fake.applyWorkdir.WorkdirPath())
}

func TestTerraformActorExec_Apply_PropagatesActorError(t *testing.T) {
	fake := &fakeTfActor{applyErr: assertErr}
	adapter := NewTerraformActorExec(fake, nil, nil)

	out, err := adapter.Apply(context.Background(), "run-1", []byte(`{}`))
	require.ErrorIs(t, err, assertErr)
	require.Nil(t, out)
}

func TestTerraformActorExec_Destroy_CallsDestroyExistingWithDerivedWdId(t *testing.T) {
	fake := &fakeTfActor{}
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	env := map[string]string{"YC_TOKEN": "secret"}
	adapter := NewTerraformActorExec(fake, tfFiles, env)

	err := adapter.Destroy(context.Background(), "run-1", []byte(`{"stroppy_nodes":[]}`))
	require.NoError(t, err)

	require.Equal(t, terraform.NewWdId("run-1"), fake.destroyWd)
	require.NotEmpty(t, fake.destroyOpts, "destroy must pass the same file/var/env option set as apply")

	// Applying the captured options to a fresh WorkdirWithParams must produce
	// the same WorkdirPath as apply's, confirming Destroy derives its WdId
	// from dir the same way.
	w := terraform.NewWorkdirWithParams(fake.destroyWd, fake.destroyOpts...)
	require.Equal(t, terraform.NewWorkdirWithParams(terraform.NewWdId("run-1")).WorkdirPath(), w.WorkdirPath())
}

func TestTerraformActorExec_Destroy_PropagatesActorError(t *testing.T) {
	fake := &fakeTfActor{destroyErr: assertErr}
	adapter := NewTerraformActorExec(fake, nil, nil)

	err := adapter.Destroy(context.Background(), "run-1", []byte(`{}`))
	require.ErrorIs(t, err, assertErr)
}

var _ terraformExec = (*terraformActorExec)(nil)
