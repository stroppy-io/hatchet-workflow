package workflows

import (
	"sort"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// deriveRecipeTopology builds the models.RecipeTopologySnapshot RunRecipeWorkflow
// persists onto TestRunRecord.recipe_topology (see that field's doc) right
// after ProvisionActivity succeeds (see run's infra-stage block in
// runrecipe.go). machines is ProvisionActivityOutput.Machines, keyed by the
// plan's machine_groups entry name exactly as the provider returned it (see
// provider.docker/terraform's own map[group.Name][]*MachineState shape) —
// this is the ONLY input needed to know which group each machine belongs to;
// plan.GetServices() supplies which dsl.ServiceSpec entries are on_group
// placed on that same group name. Deterministic: machines are emitted sorted
// by (group, node_id) regardless of map iteration order.
func deriveRecipeTopology(plan *dslpb.CompiledPlan, machines map[string][]*deploymentpb.MachineState) *models.RecipeTopologySnapshot {
	svcByGroup := servicesByGroup(plan.GetServices())

	groups := make([]string, 0, len(machines))
	for group := range machines {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	nodes := make([]*models.RecipeTopologySnapshot_MachineNode, 0)
	for _, group := range groups {
		groupServices := serviceNodesFor(svcByGroup[group])

		groupMachines := append([]*deploymentpb.MachineState(nil), machines[group]...)
		sort.SliceStable(groupMachines, func(i, j int) bool {
			return groupMachines[i].GetNodeId() < groupMachines[j].GetNodeId()
		})

		for _, machine := range groupMachines {
			if machine.GetNodeId() == "" {
				continue
			}
			nodes = append(nodes, &models.RecipeTopologySnapshot_MachineNode{
				NodeId:   machine.GetNodeId(),
				Group:    group,
				Ip:       recipeMachineAddress(machine),
				Status:   machine.GetStatus(),
				Services: groupServices,
				Labels:   machine.GetLabels(),
			})
		}
	}

	return &models.RecipeTopologySnapshot{
		Provider: plan.GetProvider().GetName(),
		Nodes:    nodes,
	}
}

// servicesByGroup buckets a compiled plan's services by their on_group,
// preserving the plan's own declaration order within each group.
func servicesByGroup(services []*dslpb.ServiceSpec) map[string][]*dslpb.ServiceSpec {
	out := make(map[string][]*dslpb.ServiceSpec)
	for _, svc := range services {
		group := svc.GetOnGroup()
		if group == "" {
			continue
		}
		out[group] = append(out[group], svc)
	}
	return out
}

// serviceNodesFor projects a group's ServiceSpecs into the snapshot's
// ServiceNode shape (name + image only — everything else a ServiceSpec
// carries, e.g. env/volumes/health, is deployment detail the topology view
// does not need).
func serviceNodesFor(services []*dslpb.ServiceSpec) []*models.RecipeTopologySnapshot_ServiceNode {
	if len(services) == 0 {
		return nil
	}
	nodes := make([]*models.RecipeTopologySnapshot_ServiceNode, 0, len(services))
	for _, svc := range services {
		nodes = append(nodes, &models.RecipeTopologySnapshot_ServiceNode{
			Name:  svc.GetName(),
			Image: svc.GetImage(),
		})
	}
	return nodes
}

// recipeMachineAddress picks a machine's primary address (private endpoint
// preferred, else the first available) — duplicated in spirit from
// execution.machineHost (internal/infrastructure/execution/overview.go)
// rather than shared: internal/workflows cannot import
// internal/infrastructure/execution without an import cycle (execution
// already imports workflows for the RunRecipeInput/CompileRecipeActivityInput
// etc. types this file's sibling runrecipe.go declares).
func recipeMachineAddress(machine *deploymentpb.MachineState) string {
	for _, name := range []string{"private", "public"} {
		for _, endpoint := range machine.GetEndpoints() {
			if endpoint.GetName() == name && endpoint.GetAddress() != "" {
				return endpoint.GetAddress()
			}
		}
	}
	for _, endpoint := range machine.GetEndpoints() {
		if endpoint.GetAddress() != "" {
			return endpoint.GetAddress()
		}
	}
	return ""
}
