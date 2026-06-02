// Package dbtest holds shared helpers for engine topology/deployment tests:
// a fake infrastructure state and step/artifact finders.
package dbtest

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// InfrastructureStateForSpec builds a deployed Docker infrastructure state with
// one private endpoint per node, addressed 10.0.0.1, 10.0.0.2, ...
func InfrastructureStateForSpec(spec *topologypb.TopologySpec) *deploymentpb.InfrastructureState {
	state := &deploymentpb.InfrastructureState{
		Provider: deploymentpb.Provider_PROVIDER_DOCKER,
		Machines: make([]*deploymentpb.MachineState, 0, len(spec.GetNodes())),
		Tags:     &common.Tags{Tags: []string{"test"}},
	}

	for i, node := range spec.GetNodes() {
		privatePort := uint32(22)
		state.Machines = append(state.Machines, &deploymentpb.MachineState{
			NodeId:             node.GetId(),
			ProviderResourceId: "container-" + node.GetId(),
			Status:             common.Status_STATUS_DEPLOYED,
			Endpoints: []*deploymentpb.Endpoint{
				{
					Name:    "private",
					Address: "10.0.0." + string(rune('1'+i)),
					Port:    &privatePort,
				},
			},
		})
	}

	return state
}

// ComponentsByID indexes a deployment plan by component id.
func ComponentsByID(plan *deploymentpb.DeploymentPlan) map[string]*deploymentpb.ComponentDeployment {
	components := make(map[string]*deploymentpb.ComponentDeployment, len(plan.GetComponents()))
	for _, component := range plan.GetComponents() {
		components[component.GetComponentId()] = component
	}
	return components
}

// WriteFileText returns the text written by the WriteFile step with the given id.
func WriteFileText(component *deploymentpb.ComponentDeployment, stepID string) string {
	for _, step := range component.GetSteps() {
		if step.GetId() == stepID {
			return step.GetWriteFile().GetText()
		}
	}
	return ""
}

// CallCmd returns the script of the CallCmd step with the given id.
func CallCmd(component *deploymentpb.ComponentDeployment, stepID string) string {
	for _, step := range component.GetSteps() {
		if step.GetId() == stepID {
			return step.GetCallCmd().GetSpec().GetScript().GetText()
		}
	}
	return ""
}

// ServiceUnitText returns the systemd unit text (200_write_service step).
func ServiceUnitText(component *deploymentpb.ComponentDeployment) string {
	return WriteFileText(component, "200_write_service")
}

// ComponentsByIDInSpec indexes a topology spec by component id.
func ComponentsByIDInSpec(spec *topologypb.TopologySpec) map[string]*topologypb.Component {
	components := make(map[string]*topologypb.Component, len(spec.GetComponents()))
	for _, component := range spec.GetComponents() {
		components[component.GetId()] = component
	}
	return components
}

// HasConnection reports whether the spec has a from→to connection with the
// given endpoint name.
func HasConnection(spec *topologypb.TopologySpec, from, to, endpoint string) bool {
	for _, connection := range spec.GetConnections() {
		if connection.GetFromComponentId() == from &&
			connection.GetToComponentId() == to &&
			connection.GetEndpointName() == endpoint {
			return true
		}
	}
	return false
}
