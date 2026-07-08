package models_test

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestRun_ProtojsonRoundTrip(t *testing.T) {
	run := &models.Run{
		Entity:     &commonpb.Entity{Id: "run-1", TenantId: "tenant-1"},
		Status:     commonpb.Status_STATUS_RUNNING,
		WorkflowId: "recipe-1",
		Topology: &models.RunTopology{
			Provider: "docker",
			Nodes: []*models.RunTopology_MachineNode{
				{NodeId: "db-0", Group: "db", Status: commonpb.Status_STATUS_COMPLETED},
			},
		},
		Observability: &models.ObservabilityRefs{MetricsQueryKey: "run-1", LogsQueryKey: "run-1"},
		Summary:       &models.Run_Summary{DbKind: 0, TopologyLabel: "PG x1"},
	}

	data, err := protojson.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := &models.Run{}
	if err := protojson.Unmarshal(data, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.GetTopology().GetNodes()[0].GetNodeId() != "db-0" {
		t.Fatalf("round-trip lost topology.nodes[0].node_id")
	}
	if got.GetWorkflowId() != "recipe-1" {
		t.Fatalf("round-trip lost workflow_id")
	}
}
