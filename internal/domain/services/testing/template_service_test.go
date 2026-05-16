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

func TestTemplateCRUD(t *testing.T) {
	iam := fixture.NewIAM(t)
	executor := iam.F.Executor.(*sqlexec.TxExecutor)
	svc := testingsvc.NewTemplateService(executor, iam.F.TxMgr, iam.F.Events)
	ctx := context.Background()

	u, _ := iam.IAM.CreateUser(ctx, &iampb.User{Email: "tpl@e.com", Nickname: "tpl"}, "P@ss1234!")
	tn, _ := iam.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())

	tpl := &testingpb.TestRunTemplate{Identity: &commonpb.Identity{Name: "tpc-c basic"}}
	created, err := svc.CreateTestRunTemplate(ctx, tn.GetId(), u.GetId(), tpl)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	got, err := svc.GetTestRunTemplate(ctx, created.GetId())
	require.NoError(t, err)
	require.Equal(t, created.GetId().GetValue(), got.GetId().GetValue())

	list, err := svc.ListTestRunTemplates(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, svc.DeleteTestRunTemplate(ctx, created.GetId()))
	list2, err := svc.ListTestRunTemplates(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list2, 0)
}
