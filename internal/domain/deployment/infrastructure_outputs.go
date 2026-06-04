package deployment

import (
	"fmt"
	"regexp"
	"strings"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var sensitiveJSONFieldRE = regexp.MustCompile(`(?i)("([^"]*(token|password|secret|privateKey|private_key)[^"]*)"\s*:\s*)"[^"]*"`)

// InfrastructurePlanOutputs exposes the provider-facing infrastructure intent
// that Temporal is about to materialize: provider settings, machine provider
// params, and requested quotas.
func InfrastructurePlanOutputs(plan *deploymentpb.InfrastructurePlan) []*monitor.PipelineOutput {
	if plan == nil {
		return nil
	}
	provider := providerLabel(plan.GetProvider())
	machineCount := len(plan.GetMachines())
	quotaCount := 0
	for _, machine := range plan.GetMachines() {
		quotaCount += len(machine.GetQuotaRequests())
	}
	outputs := []*monitor.PipelineOutput{
		{
			Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:             "infrastructure/plan",
			Name:           "infrastructure plan",
			Summary:        fmt.Sprintf("provider %s, %d machines, %d quota requests", provider, machineCount, quotaCount),
			Target:         provider,
			ContentPreview: protoPreview(plan),
			Count:          uint32(machineCount),
			Labels:         cloneStringMap(plan.GetLabels()),
		},
	}
	if settings := plan.GetSettings(); settings != nil {
		outputs = append(outputs, &monitor.PipelineOutput{
			Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:             "infrastructure/provider_settings",
			Name:           "provider settings",
			Summary:        "settings for provider " + provider,
			Target:         provider,
			ContentPreview: protoPreview(settings),
			Labels: map[string]string{
				"provider": provider,
			},
		})
	}
	for _, machine := range plan.GetMachines() {
		if machine == nil || len(outputs) >= maxPipelineOutputs {
			continue
		}
		outputs = append(outputs, &monitor.PipelineOutput{
			Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:             "infrastructure/machine/" + machine.GetNodeId(),
			Name:           machine.GetNodeId(),
			Summary:        fmt.Sprintf("%s provider params, %d quota requests", provider, len(machine.GetQuotaRequests())),
			MachineId:      machine.GetNodeId(),
			Target:         machine.GetNodeId(),
			ContentPreview: protoPreview(machine),
			Count:          uint32(len(machine.GetQuotaRequests())),
			Labels:         cloneStringMap(machine.GetLabels()),
		})
		for _, req := range machine.GetQuotaRequests() {
			if req == nil || len(outputs) >= maxPipelineOutputs {
				break
			}
			outputs = append(outputs, quotaRequestOutput(machine.GetNodeId(), req))
		}
	}
	return outputs
}

// QuotaRequestRefOutputs exposes the normalized quota request list returned by
// CalculateQuotasWorkflow, including the node each request belongs to.
func QuotaRequestRefOutputs(refs []*workflowpb.QuotaRequestRef) []*monitor.PipelineOutput {
	outputs := make([]*monitor.PipelineOutput, 0, len(refs)+1)
	outputs = append(outputs, &monitor.PipelineOutput{
		Kind:    monitor.OutputKind_OUTPUT_KIND_SUMMARY,
		Id:      "quota/requests",
		Name:    "quota requests",
		Summary: fmt.Sprintf("%d quota requests", len(refs)),
		Count:   uint32(len(refs)),
	})
	for _, ref := range refs {
		if ref == nil || ref.GetRequest() == nil || len(outputs) >= maxPipelineOutputs {
			continue
		}
		outputs = append(outputs, quotaRequestOutput(ref.GetNodeId(), ref.GetRequest()))
	}
	return outputs
}

// QuotaAllocationRefOutputs exposes provider quota reservations/allocations
// returned by acquire/commit activities.
func QuotaAllocationRefOutputs(refs []*workflowpb.QuotaAllocationRef) []*monitor.PipelineOutput {
	outputs := make([]*monitor.PipelineOutput, 0, len(refs)+1)
	outputs = append(outputs, &monitor.PipelineOutput{
		Kind:    monitor.OutputKind_OUTPUT_KIND_SUMMARY,
		Id:      "quota/allocations",
		Name:    "quota allocations",
		Summary: fmt.Sprintf("%d quota allocations", len(refs)),
		Count:   uint32(len(refs)),
	})
	for _, ref := range refs {
		if ref == nil || ref.GetAllocation() == nil || len(outputs) >= maxPipelineOutputs {
			continue
		}
		allocation := ref.GetAllocation()
		info := allocation.GetInfo()
		outputs = append(outputs, &monitor.PipelineOutput{
			Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:             "quota/allocation/" + ref.GetNodeId() + "/" + info.GetName(),
			Name:           info.GetName(),
			Summary:        fmt.Sprintf("%s allocated %d %s on %s", info.GetName(), allocation.GetUsed(), info.GetUnits(), ref.GetNodeId()),
			MachineId:      ref.GetNodeId(),
			Target:         info.GetName(),
			ContentPreview: protoPreview(allocation),
			Count:          uint32(allocation.GetUsed()),
			Labels: map[string]string{
				"provider": providerLabel(info.GetProvider()),
				"quota":    info.GetName(),
				"units":    info.GetUnits(),
			},
		})
	}
	return outputs
}

// InfrastructureStateOutputs exposes provider runtime output after materialize:
// machine resource ids, endpoints, provider outputs, and allocated quotas.
func InfrastructureStateOutputs(state *deploymentpb.InfrastructureState) []*monitor.PipelineOutput {
	if state == nil {
		return nil
	}
	provider := providerLabel(state.GetProvider())
	outputs := []*monitor.PipelineOutput{
		{
			Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:             "infrastructure/state",
			Name:           "infrastructure state",
			Summary:        fmt.Sprintf("provider %s deployed %d machines", provider, len(state.GetMachines())),
			Target:         provider,
			ContentPreview: protoPreview(state),
			Count:          uint32(len(state.GetMachines())),
			Labels:         cloneStringMap(state.GetLabels()),
		},
	}
	for _, machine := range state.GetMachines() {
		if machine == nil || len(outputs) >= maxPipelineOutputs {
			continue
		}
		outputs = append(outputs, &monitor.PipelineOutput{
			Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
			Id:             "infrastructure/state/machine/" + machine.GetNodeId(),
			Name:           machine.GetNodeId(),
			Summary:        fmt.Sprintf("%s resource %s, %d endpoints, %d quota allocations", provider, machine.GetProviderResourceId(), len(machine.GetEndpoints()), len(machine.GetAllocatedQuotas())),
			MachineId:      machine.GetNodeId(),
			Target:         machine.GetProviderResourceId(),
			ContentPreview: protoPreview(machine),
			Count:          uint32(len(machine.GetEndpoints())),
			Labels:         cloneStringMap(machine.GetLabels()),
		})
		for _, allocation := range machine.GetAllocatedQuotas() {
			if allocation == nil || len(outputs) >= maxPipelineOutputs {
				break
			}
			info := allocation.GetInfo()
			outputs = append(outputs, &monitor.PipelineOutput{
				Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
				Id:             "infrastructure/state/machine/" + machine.GetNodeId() + "/quota/" + info.GetName(),
				Name:           info.GetName(),
				Summary:        fmt.Sprintf("%s uses %d %s on %s", info.GetName(), allocation.GetUsed(), info.GetUnits(), machine.GetNodeId()),
				MachineId:      machine.GetNodeId(),
				Target:         info.GetName(),
				ContentPreview: protoPreview(allocation),
				Count:          uint32(allocation.GetUsed()),
				Labels: map[string]string{
					"provider": providerLabel(info.GetProvider()),
					"quota":    info.GetName(),
					"units":    info.GetUnits(),
				},
			})
		}
	}
	return outputs
}

func NetworkCIDROutput(id, name, cidr string) *monitor.PipelineOutput {
	if cidr == "" {
		return nil
	}
	return &monitor.PipelineOutput{
		Kind:    monitor.OutputKind_OUTPUT_KIND_SUMMARY,
		Id:      id,
		Name:    name,
		Summary: "network CIDR " + cidr,
		Target:  cidr,
		Labels: map[string]string{
			"network_cidr": cidr,
		},
	}
}

func quotaRequestOutput(nodeID string, req *deploymentpb.Quota_Request) *monitor.PipelineOutput {
	info := req.GetInfo()
	return &monitor.PipelineOutput{
		Kind:           monitor.OutputKind_OUTPUT_KIND_SUMMARY,
		Id:             "quota/request/" + nodeID + "/" + info.GetName(),
		Name:           info.GetName(),
		Summary:        fmt.Sprintf("%s requests %d %s on %s", info.GetName(), req.GetRequest(), info.GetUnits(), nodeID),
		MachineId:      nodeID,
		Target:         info.GetName(),
		ContentPreview: protoPreview(req),
		Count:          uint32(req.GetRequest()),
		Labels: map[string]string{
			"provider": providerLabel(info.GetProvider()),
			"quota":    info.GetName(),
			"units":    info.GetUnits(),
		},
	}
}

func protoPreview(msg proto.Message) string {
	if msg == nil {
		return ""
	}
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
	if err != nil {
		return ""
	}
	return redactSensitivePreview(boundedString(string(data), 4096))
}

func redactSensitivePreview(preview string) string {
	return sensitiveJSONFieldRE.ReplaceAllString(preview, `${1}"***"`)
}

func providerLabel(provider deploymentpb.Provider) string {
	label := strings.TrimPrefix(provider.String(), "PROVIDER_")
	return strings.ToLower(label)
}
