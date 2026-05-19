//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func twoCellMatrix() *testingpb.TestSuite_Matrix {
	return &testingpb.TestSuite_Matrix{
		Databases: []*catalogpb.DatabaseOrPreset{pgInlineDB(), pgInlineDB()},
		Workloads: []*catalogpb.WorkloadOrPreset{inlineWorkload()},
	}
}

func TestE2E_TestSuite_CRUD_WithMatrix(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "suite-crud")
	cli := env.WithTenant(tnt.GetValue())

	created, err := cli.TestSuite.CreateTestSuite(ctx, connect.NewRequest(&testingpb.CreateTestSuiteRequest{
		Suite: &testingpb.TestSuite{
			Identity: &commonpb.Identity{Name: "suite-A"},
			Matrix:   twoCellMatrix(),
		},
	}))
	require.NoError(t, err)
	id := created.Msg.GetId()
	require.Len(t, created.Msg.GetMatrix().GetDatabases(), 2)

	got, err := cli.TestSuite.GetTestSuite(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	require.Equal(t, "suite-A", got.Msg.GetIdentity().GetName())

	upd, err := cli.TestSuite.UpdateTestSuite(ctx, connect.NewRequest(&testingpb.UpdateTestSuiteRequest{
		Suite: &testingpb.TestSuite{
			Id:       id,
			Identity: &commonpb.Identity{Name: "suite-A-renamed"},
			Matrix:   twoCellMatrix(),
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "suite-A-renamed", upd.Msg.GetIdentity().GetName())

	lst, err := cli.TestSuite.ListTestSuites(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.NotEmpty(t, lst.Msg.GetTestSuites())

	_, err = cli.TestSuite.DeleteTestSuite(ctx, connect.NewRequest(id))
	require.NoError(t, err)
}

func TestE2E_TestSuite_Clone(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "suite-clone")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.TestSuite.CreateTestSuite(ctx, connect.NewRequest(&testingpb.CreateTestSuiteRequest{
		Suite: &testingpb.TestSuite{
			Identity: &commonpb.Identity{Name: "orig-suite"},
			Matrix:   twoCellMatrix(),
		},
	}))
	require.NoError(t, err)

	cloned, err := cli.TestSuite.CloneTestSuite(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	require.NotEqual(t, created.Msg.GetId().GetValue(), cloned.Msg.GetId().GetValue())
	require.Len(t, cloned.Msg.GetMatrix().GetDatabases(), 2)
}

func TestE2E_TestSuite_TenantIsolation(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tntA := env.SeedTenant(t, "suite-iso-A")
	tntB := env.SeedTenant(t, "suite-iso-B")
	a := env.WithTenant(tntA.GetValue())
	b := env.WithTenant(tntB.GetValue())

	created, err := a.TestSuite.CreateTestSuite(ctx, connect.NewRequest(&testingpb.CreateTestSuiteRequest{
		Suite: &testingpb.TestSuite{
			Identity: &commonpb.Identity{Name: "A-only"},
			Matrix:   twoCellMatrix(),
		},
	}))
	require.NoError(t, err)
	idA := created.Msg.GetId().GetValue()
	lstB, err := b.TestSuite.ListTestSuites(ctx, connect.NewRequest(tntB))
	require.NoError(t, err)
	for _, s := range lstB.Msg.GetTestSuites() {
		require.NotEqual(t, idA, s.GetId().GetValue())
	}
}
