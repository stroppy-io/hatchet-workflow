package dagbuilder

import (
	"context"
	"fmt"

	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	taskspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/tasks"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// FromTestSuite builds a meta-DAG where each child TestRun becomes one
// Dag_Node carrying a TestRunRefTask. The execution mode (parallel or
// sequential) is derived from suite.Policy.Mode:
//
//   - MODE_SEQUENTIAL (or unspecified): each node depends on the previous one.
//   - MODE_PARALLEL: all nodes have no dependencies (admitted concurrently).
func (b *Builder) FromTestSuite(
	_ context.Context,
	suite *testingpb.TestSuite,
	childIDs []*testingpb.TestRunId,
) (*systempb.Dag, error) {
	if len(childIDs) == 0 {
		return nil, fmt.Errorf("dagbuilder: FromTestSuite: no child TestRunIds provided")
	}

	sequential := true
	if p := suite.GetPolicy(); p != nil {
		sequential = p.GetMode() != testingpb.TestSuite_Policy_MODE_PARALLEL
	}

	nodes := make([]*systempb.Dag_Node, 0, len(childIDs))
	for i, id := range childIDs {
		var deps []string
		if sequential && i > 0 {
			deps = []string{fmt.Sprintf("run-%d", i-1)}
		}

		nodeID := fmt.Sprintf("run-%d", i)
		node, err := mkNode(nodeID, deps, &taskspb.TestRunRefTask{
			TestRunId: id,
		})
		if err != nil {
			return nil, fmt.Errorf("dagbuilder: FromTestSuite: node %s: %w", nodeID, err)
		}
		nodes = append(nodes, node)
	}

	return &systempb.Dag{
		Graph: &systempb.Dag_Graph{
			Nodes: nodes,
		},
	}, nil
}
