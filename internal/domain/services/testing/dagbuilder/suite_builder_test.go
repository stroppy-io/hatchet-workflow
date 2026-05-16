package dagbuilder_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func TestBuilder_FromTestSuite_Parallel(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})

	ids := []*testingpb.TestRunId{
		{Value: "run-a"},
		{Value: "run-b"},
		{Value: "run-c"},
	}
	suite := &testingpb.TestSuite{
		Policy: &testingpb.TestSuite_Policy{
			Mode: testingpb.TestSuite_Policy_MODE_PARALLEL,
		},
	}

	dag, err := b.FromTestSuite(context.Background(), suite, ids)
	require.NoError(t, err)
	require.NotNil(t, dag.GetGraph())
	require.Len(t, dag.GetGraph().GetNodes(), 3)

	for _, node := range dag.GetGraph().GetNodes() {
		require.Empty(t, node.GetDeps(), "parallel nodes must have no deps")
	}
}

func TestBuilder_FromTestSuite_Sequential(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})

	ids := []*testingpb.TestRunId{
		{Value: "run-a"},
		{Value: "run-b"},
		{Value: "run-c"},
	}
	suite := &testingpb.TestSuite{
		Policy: &testingpb.TestSuite_Policy{
			Mode: testingpb.TestSuite_Policy_MODE_SEQUENTIAL,
		},
	}

	dag, err := b.FromTestSuite(context.Background(), suite, ids)
	require.NoError(t, err)
	nodes := dag.GetGraph().GetNodes()
	require.Len(t, nodes, 3)

	// first node: no deps
	require.Empty(t, nodes[0].GetDeps())
	// subsequent nodes: chained
	require.Equal(t, []string{"run-0"}, nodes[1].GetDeps())
	require.Equal(t, []string{"run-1"}, nodes[2].GetDeps())
}

func TestBuilder_FromTestSuite_NoChildren(t *testing.T) {
	b := dagbuilder.New(fakeCatalog{})
	_, err := b.FromTestSuite(context.Background(), &testingpb.TestSuite{}, nil)
	require.Error(t, err)
}
