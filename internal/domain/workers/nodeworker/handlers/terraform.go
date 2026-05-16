// TerraformHandler is a SKELETON. The infrastructure/terraform.Actor API
// requires a running terraform binary and cloud credentials not available
// in unit tests. The handler logs intent and returns success for
// UNSPECIFIED module; real modules return an error prompting manual
// wiring before production use.
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

// TerraformHandler implements nodeworker.Handler for TerraformTask.
// This is a skeleton: it logs the operation and returns success for
// MODULE_UNSPECIFIED (test/no-op). Real APPLY/DESTROY require the
// terraform binary on PATH and valid cloud credentials in env.
type TerraformHandler struct {
	log *zap.Logger
}

// NewTerraformHandler constructs a TerraformHandler.
func NewTerraformHandler(log *zap.Logger) *TerraformHandler {
	return &TerraformHandler{log: log}
}

func (h *TerraformHandler) Kind() string { return "TerraformTask" }

func (h *TerraformHandler) Execute(ctx context.Context, node *systempb.NodeRun, spec *anypb.Any, state nodeworker.StateStore) (*anypb.Any, error) {
	var task taskspb.TerraformTask
	if err := anypb.UnmarshalTo(spec, &task, proto.UnmarshalOptions{}); err != nil {
		return nil, fmt.Errorf("TerraformHandler: unmarshal spec: %w", err)
	}
	h.log.Info("TerraformHandler.Execute (skeleton)",
		zap.String("node_run_id", node.GetId().GetValue()),
		zap.String("op", task.GetOp().String()),
		zap.String("module", task.GetModule().String()),
	)
	if task.GetModule() == taskspb.TerraformTask_MODULE_UNSPECIFIED {
		// No-op for test/unspecified module.
		return nil, nil
	}
	return nil, fmt.Errorf("TerraformHandler: real terraform execution not implemented (skeleton): module=%s op=%s", task.GetModule(), task.GetOp())
}
