// Package tasks registers the control-plane (SERVER-locus) task handlers into the
// runtime TasksRegistry: render_config, terraform_apply, terraform_destroy,
// collect_results. Agent-locus nodes (handler="agent.command") are NOT here — the
// agent executes them. Handler names are the planner's Handler*.
package tasks

import (
	"context"

	"github.com/gopherex/xlog"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/planner"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
)

// NewRegistry builds the SERVER task registry. tf runs the terraform handlers.
//
// TODO(tasks): runtime.TaskFn has no context.Context — handlers use
// context.Background(), so server tasks are not cancellable via the run ctx.
// Propagate ctx through the runtime task interface. Reported.
func NewRegistry(_ *xlog.Logger, tf *terraform.Runner) *runtime.TaskRegistry {
	return runtime.NewTaskRegistry(
		runtime.NewTask[*ops.TfOperation, *ops.TfOperation_Output](planner.HandlerTerraformApply, terraformApply(tf)),
		runtime.NewTask[*ops.TfOperation, *ops.TfOperation_Output](planner.HandlerTerraformDestroy, terraformDestroy(tf)),
		runtime.NewTask[*emptypb.Empty, *emptypb.Empty](planner.HandlerRenderConfig, renderConfig),
		runtime.NewTask[*emptypb.Empty, *emptypb.Empty](planner.HandlerCollectResults, collectResults),
	)
}

// terraformApply writes the module + runs init/apply, returning outputs_json.
func terraformApply(tf *terraform.Runner) runtime.TaskFn[*ops.TfOperation, *ops.TfOperation_Output] {
	return func(op *ops.TfOperation) (*ops.TfOperation_Output, error) {
		if op.GetInput() != nil {
			op.Input.Action = ops.TfOperation_ACTION_APPLY
		}
		return tf.Run(context.Background(), op)
	}
}

// terraformDestroy destroys an existing workdir's resources.
func terraformDestroy(tf *terraform.Runner) runtime.TaskFn[*ops.TfOperation, *ops.TfOperation_Output] {
	return func(op *ops.TfOperation) (*ops.TfOperation_Output, error) {
		if op.GetInput() != nil {
			op.Input.Action = ops.TfOperation_ACTION_DESTROY
		}
		return tf.Run(context.Background(), op)
	}
}

// renderConfig renders the Database intent into config files + terraform vars.
//
// TODO(tasks): NOT implemented — needs the render engine (Database/Topology
// intent -> RenderedConfig files + terraform.tfvars.json) so its output feeds
// terraform_apply via render.Binding. Returns empty for now. Reported.
func renderConfig(_ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// collectResults gathers run metrics after the workload finishes.
//
// TODO(tasks): NOT implemented — needs a VictoriaMetrics summary collector
// (E19). Returns empty for now. Reported.
func collectResults(_ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
