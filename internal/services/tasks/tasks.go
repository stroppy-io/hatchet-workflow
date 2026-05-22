// Package tasks registers the control-plane (SERVER-locus) task handlers into the
// runtime TasksRegistry: render_config, terraform_apply, terraform_destroy,
// collect_results. Agent-locus nodes (handler="agent.command") are NOT here — the
// agent executes them. Handler names are the dag package's Handler*.
package tasks

import (
	"github.com/gopherex/xlog"
	"google.golang.org/protobuf/types/known/emptypb"

	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
)

// NewRegistry builds the SERVER task registry. tf runs the terraform handlers.
// Handlers receive a runtime.DagContext (a context.Context that also exposes the
// dag + upstream outputs), so they are cancellable via the run ctx and read prior
// node outputs as pure proto->proto transforms.
func NewRegistry(_ *xlog.Logger, tf *terraform.Runner) *runtime.TaskRegistry {
	return runtime.NewTaskRegistry(
		runtime.NewTask[*ops.TfOperation, *ops.TfOperation_Output](dagdomain.HandlerTerraformApply, terraformApply(tf)),
		runtime.NewTask[*ops.TfOperation, *ops.TfOperation_Output](dagdomain.HandlerTerraformDestroy, terraformDestroy(tf)),
		runtime.NewTask[*emptypb.Empty, *emptypb.Empty](dagdomain.HandlerRenderConfig, renderConfig),
		runtime.NewTask[*emptypb.Empty, *emptypb.Empty](dagdomain.HandlerCollectResults, collectResults),
	)
}

// terraformApply writes the module + runs init/apply, returning outputs_json.
func terraformApply(tf *terraform.Runner) runtime.TaskFn[*ops.TfOperation, *ops.TfOperation_Output] {
	return func(ctx runtime.DagContext, op *ops.TfOperation) (*ops.TfOperation_Output, error) {
		if op.GetInput() != nil {
			op.Input.Action = ops.TfOperation_ACTION_APPLY
		}
		return tf.Run(ctx, op)
	}
}

// terraformDestroy destroys an existing workdir's resources.
func terraformDestroy(tf *terraform.Runner) runtime.TaskFn[*ops.TfOperation, *ops.TfOperation_Output] {
	return func(ctx runtime.DagContext, op *ops.TfOperation) (*ops.TfOperation_Output, error) {
		if op.GetInput() != nil {
			op.Input.Action = ops.TfOperation_ACTION_DESTROY
		}
		return tf.Run(ctx, op)
	}
}

// renderConfig renders the Database intent into config files + terraform vars.
//
// TODO(tasks): NOT implemented — needs the render engine (Database/Topology
// intent -> RenderedConfig files + terraform.tfvars.json) so its output feeds
// terraform_apply via render.Binding. Returns empty for now. Reported.
func renderConfig(_ runtime.DagContext, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// collectResults gathers run metrics after the workload finishes.
//
// TODO(tasks): NOT implemented — needs a VictoriaMetrics summary collector
// (E19). Returns empty for now. Reported.
func collectResults(_ runtime.DagContext, _ *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
