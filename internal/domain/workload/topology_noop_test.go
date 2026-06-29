package workload

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

// A PROTOCOL_NOOP workload deploys no database: the runner stands alone and
// connects to nothing, so the extended spec is exactly one runner node/component
// with no connections.
func TestExtendTopologySpecNoopIsRunnerOnly(t *testing.T) {
	workload := &domain.Workload{
		Protocol: domain.Workload_PROTOCOL_NOOP,
		Segments: []*domain.Workload_Segment{{
			Name:      "workload",
			Script:    "tpcc/tx",
			Execution: &domain.Workload_Execution{},
		}},
	}

	spec, err := ExtendTopologySpec(&topologypb.TopologySpec{}, workload)
	if err != nil {
		t.Fatalf("extend topology spec: %v", err)
	}
	if got := len(spec.GetNodes()); got != 1 {
		t.Fatalf("nodes = %d, want 1 (runner only)", got)
	}
	if got := len(spec.GetComponents()); got != 1 {
		t.Fatalf("components = %d, want 1 (runner only)", got)
	}
	if got := len(spec.GetConnections()); got != 0 {
		t.Fatalf("connections = %d, want 0 (no database to connect to)", got)
	}
	if got := spec.GetComponents()[0].GetId(); got != RunnerNodeID {
		t.Fatalf("component id = %q, want %q", got, RunnerNodeID)
	}
}
