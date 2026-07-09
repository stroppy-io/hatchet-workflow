package terraform

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithStdoutStderr_SetFields(t *testing.T) {
	var out, errOut bytes.Buffer
	w := NewWorkdirWithParams(NewWdId("run-1"), WithWorkdirStdout(&out), WithWorkdirStderr(&errOut))
	require.Same(t, &out, w.Stdout())
	require.Same(t, &errOut, w.Stderr())
}

func TestWorkdirWithParams_NoStdoutOption_NilByDefault(t *testing.T) {
	w := NewWorkdirWithParams(NewWdId("run-1"))
	require.Nil(t, w.Stdout())
	require.Nil(t, w.Stderr())
}

func TestMergeWriter_NilPerRunFallsBackToActorDefault(t *testing.T) {
	var actorDefault bytes.Buffer
	got := mergeWriter(&actorDefault, nil)
	_, err := got.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, "hello", actorDefault.String())
}

func TestMergeWriter_BothWritersReceiveOutput(t *testing.T) {
	var actorDefault, perRun bytes.Buffer
	got := mergeWriter(&actorDefault, &perRun)
	_, err := got.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, "hello", actorDefault.String(), "actor-level default (dev/debug channel) still receives output")
	require.Equal(t, "hello", perRun.String(), "per-run writer (F2's log sink) receives the same output")
}
