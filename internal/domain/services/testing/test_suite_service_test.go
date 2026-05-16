package testing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

func TestTestSuiteCRUDAndClone(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)
	svc := testingsvc.NewTestSuiteService(executor, iam.F.TxMgr, iam.F.Events)
	ctx := context.Background()

	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "suite@e.com", Nickname: "suite"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "SuiteT"}}, u.GetId())

	// Create
	suite := &testingpb.TestSuite{Identity: &commonpb.Identity{Name: "my-suite"}}
	created, err := svc.CreateTestSuite(ctx, tn.GetId(), u.GetId(), suite)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())
	require.Equal(t, "my-suite", created.GetIdentity().GetName())

	// Get
	got, err := svc.GetTestSuite(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), got.GetId().GetValue())

	// List
	list, err := svc.ListTestSuites(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)

	// Clone
	cloned, err := svc.CloneTestSuite(ctx, created.GetId(), u.GetId())
	require.NoError(t, err)
	require.NotEqual(t, created.GetId().GetValue(), cloned.GetId().GetValue())
	require.Equal(t, created.GetIdentity().GetName(), cloned.GetIdentity().GetName())

	list2, err := svc.ListTestSuites(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list2, 2)

	// Delete (returns pre-delete)
	deleted, err := svc.DeleteTestSuite(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), deleted.GetId().GetValue())

	list3, err := svc.ListTestSuites(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list3, 1)
}
