//go:build e2e

package e2e_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/testserver"
)

// TestE2E_FullStack exercises the complete login → tenant → template → testrun
// → launch → dag_run path against an in-process HTTP server with the full
// middleware chain (Auth + Tenant + ErrorMapper).
func TestE2E_FullStack(t *testing.T) {
	ctx := context.Background()

	// ── 1. Stand up in-process server with all services ─────────────────────
	f := fixture.NewAll(t, "[]", "[]") // empty GitHub stub — versions not needed

	handler := testserver.ForAll(testserver.AllDeps{
		IAM:       f.IAM,
		Catalog:   f.Catalog,
		Stroppy:   f.Stroppy,
		System:    f.System,
		Templates: f.Templates,
		Runs:      f.Runs,
		Suites:    f.Suites,
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// ── 2. Bootstrap admin user directly via service ─────────────────────────
	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "admin@e2e.com", Nickname: "admin"}, "Str0ng!Pass")
	require.NoError(t, err)

	// ── 3. Login via Connect → get tokens ───────────────────────────────────
	anon := client.New(srv.URL)
	loginResp, err := anon.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{
		Email:    "admin@e2e.com",
		Password: "Str0ng!Pass",
	}))
	require.NoError(t, err)
	access := loginResp.Msg.GetTokens().GetAccessToken()
	require.NotEmpty(t, access, "access token must be non-empty after login")

	// ── 4. Create tenant ─────────────────────────────────────────────────────
	authed := client.New(srv.URL, client.WithBearer(access))
	tenantResp, err := authed.Tenant.CreateTenant(ctx, connect.NewRequest(&iampb.CreateTenantRequest{
		Tenant: &iampb.Tenant{Identity: &commonpb.Identity{Name: "SmokeOrg"}},
	}))
	require.NoError(t, err)
	tenantID := tenantResp.Msg.GetId().GetValue()
	require.NotEmpty(t, tenantID, "tenant ID must be non-empty")

	// Helper: requests that need a tenant scope carry X-Tenant-Id header.
	withTenant := client.New(srv.URL, client.WithBearer(access), client.WithTenantHeader(tenantID))

	// ── 5. Create a TestRunTemplate with inline Database + Workload ──────────
	tplResp, err := withTenant.Template.CreateTestRunTemplate(ctx, connect.NewRequest(&testingpb.CreateTestRunTemplateRequest{
		Template: &testingpb.TestRunTemplate{
			Identity: &commonpb.Identity{Name: "smoke-template"},
			Database: &catalogpb.DatabaseOrPreset{
				DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
					Database: &catalogpb.Database{
						Kind: catalogpb.Database_DATABASE_KIND_POSTGRES,
					},
				},
			},
			Workload: &catalogpb.WorkloadOrPreset{
				WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
					Workload: &catalogpb.Workload{},
				},
			},
		},
	}))
	require.NoError(t, err)
	tplID := tplResp.Msg.GetId().GetValue()
	require.NotEmpty(t, tplID, "template ID must be non-empty")

	// ── 6. Create a TestRun ──────────────────────────────────────────────────
	runResp, err := withTenant.TestRun.CreateTestRun(ctx, connect.NewRequest(&testingpb.CreateTestRunRequest{
		TestRun: &testingpb.TestRun{
			Identity: &commonpb.Identity{Name: "smoke-run"},
			Database: &catalogpb.DatabaseOrPreset{
				DatabaseVariant: &catalogpb.DatabaseOrPreset_Database{
					Database: &catalogpb.Database{
						Kind: catalogpb.Database_DATABASE_KIND_POSTGRES,
					},
				},
			},
			Workload: &catalogpb.WorkloadOrPreset{
				WorkloadVariant: &catalogpb.WorkloadOrPreset_Workload{
					Workload: &catalogpb.Workload{},
				},
			},
		},
	}))
	require.NoError(t, err)
	runID := runResp.Msg.GetId().GetValue()
	require.NotEmpty(t, runID, "test run ID must be non-empty")

	// DagRunId must be nil before launch.
	require.Nil(t, runResp.Msg.GetDagRunId(), "DagRunId must be nil before launch")

	// ── 7. Launch the TestRun ────────────────────────────────────────────────
	launchResp, err := withTenant.TestRun.LaunchTestRun(ctx, connect.NewRequest(&testingpb.TestRunId{
		Value: runID,
	}))
	require.NoError(t, err)
	require.NotNil(t, launchResp.Msg.GetDagRunId(), "DagRunId must be set after launch")
	dagRunID := launchResp.Msg.GetDagRunId().GetValue()
	require.NotEmpty(t, dagRunID, "DagRunId value must be non-empty")

	// ── 8. GetTestRun confirms DagRunId persisted ────────────────────────────
	getResp, err := withTenant.TestRun.GetTestRun(ctx, connect.NewRequest(&testingpb.TestRunId{
		Value: runID,
	}))
	require.NoError(t, err)
	require.Equal(t, dagRunID, getResp.Msg.GetTestRun().GetDagRunId().GetValue(),
		"GetTestRun must return the same DagRunId that LaunchTestRun returned")

	// ── 9. Verify DagRun exists in DB via system service directly ────────────
	dr, err := f.System.GetDagRun(ctx, &systempb.DagRunId{Value: dagRunID})
	require.NoError(t, err)
	require.NotNil(t, dr, "DagRun must exist in system DB after launch")
	require.Equal(t, dagRunID, dr.GetId().GetValue())
}
