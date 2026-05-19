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

func pgInlineDB() *catalogpb.DatabaseOrPreset {
	return &catalogpb.DatabaseOrPreset{
		DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
			Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
		},
	}
}

func inlineWorkload() *catalogpb.WorkloadOrPreset {
	return &catalogpb.WorkloadOrPreset{
		WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{Workload: &catalogpb.Workload{}},
	}
}

func TestE2E_Templates_CRUD(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "tpl-tenant")
	cli := env.WithTenant(tnt.GetValue())

	created, err := cli.Template.CreateTestRunTemplate(ctx, connect.NewRequest(&testingpb.CreateTestRunTemplateRequest{
		Template: &testingpb.TestRunTemplate{
			Identity: &commonpb.Identity{Name: "tpl-1"},
			Database: pgInlineDB(),
			Workload: inlineWorkload(),
		},
	}))
	require.NoError(t, err)
	id := created.Msg.GetId()

	got, err := cli.Template.GetTestRunTemplate(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	require.Equal(t, "tpl-1", got.Msg.GetIdentity().GetName())

	upd, err := cli.Template.UpdateTestRunTemplate(ctx, connect.NewRequest(&testingpb.UpdateTestRunTemplateRequest{
		Template: &testingpb.TestRunTemplate{
			Id:       id,
			Identity: &commonpb.Identity{Name: "tpl-renamed"},
			Database: pgInlineDB(),
			Workload: inlineWorkload(),
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "tpl-renamed", upd.Msg.GetIdentity().GetName())

	lst, err := cli.Template.ListTestRunTemplates(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	require.NotEmpty(t, lst.Msg.GetTestRunTemplates())

	_, err = cli.Template.DeleteTestRunTemplate(ctx, connect.NewRequest(id))
	require.NoError(t, err)
}

func TestE2E_Templates_InstantiateCreatesTestRunWithSnapshot(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "tpl-inst")
	cli := env.WithTenant(tnt.GetValue())

	tpl, err := cli.Template.CreateTestRunTemplate(ctx, connect.NewRequest(&testingpb.CreateTestRunTemplateRequest{
		Template: &testingpb.TestRunTemplate{
			Identity: &commonpb.Identity{Name: "tpl-inst"},
			Database: pgInlineDB(),
			Workload: inlineWorkload(),
		},
	}))
	require.NoError(t, err)

	run, err := cli.TestRun.InstantiateTestRun(ctx, connect.NewRequest(tpl.Msg.GetId()))
	require.NoError(t, err)
	require.NotEmpty(t, run.Msg.GetId().GetValue())
	require.Equal(t, tpl.Msg.GetId().GetValue(), run.Msg.GetTemplateId().GetValue())
	require.NotNil(t, run.Msg.GetDatabase(), "instantiated TestRun must carry a Database snapshot")
	require.NotNil(t, run.Msg.GetWorkload(), "instantiated TestRun must carry a Workload snapshot")
}
