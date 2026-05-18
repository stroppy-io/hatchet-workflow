// Real Docker handler. OP_UP renders the supplied files to a temp dir and
// shells `docker compose up -d`; OP_DOWN runs `docker compose down`.
// Compose path lets us reuse the same templates main used for local agent
// deployments without re-implementing the Docker SDK lifecycle.
package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// DockerHandler launches docker-compose projects. The compose project name
// is stored in the DagRun state-store under task.container_ids_state_key
// so OP_DOWN can locate the workdir on a different worker / after restart.
type DockerHandler struct {
	workRoot string
	log      *zap.Logger
}

// NewDockerHandler constructs a DockerHandler. workRoot is the parent dir
// where each task gets a sibling directory; "" picks os.TempDir().
func NewDockerHandler(workRoot string, log *zap.Logger) *DockerHandler {
	if workRoot == "" {
		workRoot = filepath.Join(os.TempDir(), "stroppy-docker")
	}
	return &DockerHandler{workRoot: workRoot, log: log}
}

// Kind matches the Any.type_url suffix the registry filters on.
func (h *DockerHandler) Kind() string { return "DockerTask" }

// Execute up/downs the project.
func (h *DockerHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.DockerTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("docker: unmarshal spec: %w", err)
	}
	stateKey := task.GetContainerIdsStateKey()
	if stateKey == "" {
		stateKey = "docker.project"
	}
	switch task.GetOp() {
	case taskspb.DockerTask_OP_UP:
		return h.up(ctx, node, &task, stateKey, state)
	case taskspb.DockerTask_OP_DOWN:
		return h.down(ctx, &task, stateKey, state)
	default:
		return nil, fmt.Errorf("docker: unsupported op %s", task.GetOp())
	}
}

func (h *DockerHandler) up(ctx context.Context, node *systempb.NodeRun, task *taskspb.DockerTask, stateKey string, state nodeworker.StateStore) (*anypb.Any, error) {
	projectDir := filepath.Join(h.workRoot, node.GetDagRunId().GetValue(), node.GetId().GetValue())
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		return nil, fmt.Errorf("docker: mkdir %s: %w", projectDir, err)
	}
	for _, f := range task.GetFiles() {
		path := filepath.Join(projectDir, filepath.Base(f.GetPath()))
		if err := os.WriteFile(path, []byte(f.GetInline()), 0o640); err != nil {
			return nil, fmt.Errorf("docker: write %s: %w", path, err)
		}
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
	cmd.Dir = projectDir
	cmd.Env = mergeEnv(task.GetVars())
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("docker compose up: %w (output: %s)", err, string(out))
	}
	if _, err := state.Put(ctx, stateKey, mustWrapString(projectDir)); err != nil {
		h.log.Warn("docker: persist project dir failed", zap.Error(err))
	}
	return nil, nil
}

func (h *DockerHandler) down(ctx context.Context, _ *taskspb.DockerTask, stateKey string, state nodeworker.StateStore) (*anypb.Any, error) {
	v, ok, err := state.Get(ctx, stateKey)
	if err != nil {
		return nil, fmt.Errorf("docker: read state %s: %w", stateKey, err)
	}
	if !ok || v == nil {
		return nil, nil
	}
	var sv structpb.Value
	_ = anypb.UnmarshalTo(v, &sv, proto.UnmarshalOptions{})
	projectDir := sv.GetStringValue()
	if projectDir == "" {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "down", "-v")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		h.log.Warn("docker compose down failed; project dir kept",
			zap.String("dir", projectDir), zap.Error(err), zap.String("out", string(out)))
	}
	_ = os.RemoveAll(projectDir)
	return nil, nil
}

func mergeEnv(vars *structpb.Struct) []string {
	env := os.Environ()
	for k, v := range vars.GetFields() {
		env = append(env, fmt.Sprintf("%s=%v", k, v.AsInterface()))
	}
	return env
}
