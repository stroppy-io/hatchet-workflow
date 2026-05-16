package testing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	testingsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/testing/dagbuilder"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/yaroher/ratel/pkg/pgx-ext/sqlexec"
)

func TestTestSuiteRunService_LaunchAndGet(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)
	ctx := context.Background()

	sys := system.New(executor, iam.F.TxMgr, iam.F.Events)
	catalog := fakeCatalog{}
	b := dagbuilder.New(catalog)

	suiteSvc := testingsvc.NewTestSuiteService(executor, iam.F.TxMgr, iam.F.Events)
	suiteRunSvc := testingsvc.NewTestSuiteRunService(executor, iam.F.TxMgr, iam.F.Events, sys, b)

	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "sr@e.com", Nickname: "sr"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "SRT"}}, u.GetId())

	// Create a suite with empty matrix (placeholder path).
	suite := &testingpb.TestSuite{
		Identity: &commonpb.Identity{Name: "launch-suite"},
		Policy: &testingpb.TestSuite_Policy{
			Mode: testingpb.TestSuite_Policy_MODE_PARALLEL,
		},
	}
	created, err := suiteSvc.CreateTestSuite(ctx, tn.GetId(), u.GetId(), suite)
	require.NoError(t, err)

	// Launch.
	suiteRun, err := suiteRunSvc.LaunchTestSuite(ctx, created.GetId(), u.GetId())
	require.NoError(t, err)
	require.NotEmpty(t, suiteRun.GetId().GetValue())
	require.NotNil(t, suiteRun.GetDagRunId(), "DagRunId must be set after launch")
	require.NotEmpty(t, suiteRun.GetTestRunIds(), "at least 1 child TestRun must be created")

	// GetTestSuiteRun round-trip.
	got, err := suiteRunSvc.GetTestSuiteRun(ctx, suiteRun.GetId())
	require.NoError(t, err)
	require.Equal(t, suiteRun.GetId().GetValue(), got.GetId().GetValue())
	require.NotEmpty(t, got.GetTestRunIds())

	// ListByTenant.
	list, err := suiteRunSvc.ListByTenant(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)
}
