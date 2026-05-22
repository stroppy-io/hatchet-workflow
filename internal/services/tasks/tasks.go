// Package tasks builds the control-plane (SERVER-locus) task registry. The whole
// provision/teardown surface is the blueprint dag's named handlers (prepareDeployment,
// deployDocker, deployYc, destroyDocker, destroyYc) registered by
// dag.RegisterProvisionHandlers with the real Deps (docker + terraform provisioners).
// Agent-locus nodes (handler="agent.command") are NOT here — the agent executes them.
package tasks

import (
	"github.com/gopherex/xlog"

	dagdomain "github.com/stroppy-io/stroppy-cloud/internal/domain/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
)

// NewRegistry builds the SERVER task registry from the provision Deps. The handlers
// receive a runtime.DagContext (a context.Context that also exposes the dag + upstream
// node outputs), so they are cancellable via the run ctx and read prior node outputs
// as pure proto->proto transforms.
func NewRegistry(_ *xlog.Logger, deps dagdomain.Deps) *runtime.TaskRegistry {
	reg := runtime.NewTaskRegistry()
	dagdomain.RegisterProvisionHandlers(reg, deps)
	return reg
}
