// DockerHandler is a SKELETON. Logs and returns success (no-op).
// Real implementation would use the Docker daemon API.
package handlers

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/workers/nodeworker"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
)

// DockerHandler handles DockerTask (skeleton — logs and returns success).
type DockerHandler struct{ log *zap.Logger }

func NewDockerHandler(log *zap.Logger) *DockerHandler { return &DockerHandler{log: log} }
func (h *DockerHandler) Kind() string                 { return "DockerTask" }
func (h *DockerHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.DockerTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("DockerHandler: unmarshal: %w", err)
	}
	h.log.Info("DockerHandler.Execute (skeleton)",
		zap.String("node_run_id", node.GetId().GetValue()),
		zap.String("op", task.GetOp().String()),
	)
	return nil, nil
}
