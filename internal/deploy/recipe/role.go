package recipe

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// KindToRole maps a topology Component kind back to the expand role that drives
// the recipe step builders. The inverse of the wizard's roleToKind. Unknown
// kinds return an empty role (no steps).
func KindToRole(kind topology.Component_Kind) expand.Role {
	switch kind {
	case topology.Component_KIND_DATABASE:
		return expand.RoleDatabase
	case topology.Component_KIND_REPLICA:
		return expand.RoleReplica
	case topology.Component_KIND_COORDINATOR:
		return expand.RoleCoordinator
	case topology.Component_KIND_PROXY:
		return expand.RoleProxy
	case topology.Component_KIND_WORKLOAD:
		return expand.RoleWorkload
	default:
		return ""
	}
}

// InstanceRole returns the primary role of a topology instance: the role of its
// first component. For the 1-machine-1-role model each instance hosts exactly one
// component, so this is unambiguous. Returns "" when the instance has no
// recognised component.
func InstanceRole(inst *topology.Topology_Instance) expand.Role {
	for _, c := range inst.GetComponents() {
		if r := KindToRole(c.GetKind()); r != "" {
			return r
		}
	}
	return ""
}
