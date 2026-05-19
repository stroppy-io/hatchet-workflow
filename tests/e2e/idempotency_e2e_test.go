//go:build e2e

package e2e_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/testserver"
)

// TestE2E_Idempotency_DuplicateBlocked exercises the X-Idempotency-Key
// middleware end-to-end. The fixture's testserver mounts the middleware with
// Enabled=true so the second call with the same key surfaces
// CodeAlreadyExists.
func TestE2E_Idempotency_DuplicateBlocked(t *testing.T) {
	ctx := context.Background()
	f := fixture.NewAll(t, "[]", "[]")
	srv := httptest.NewServer(testserver.ForAll(testserver.AllDeps{
		IAM:                f.IAM,
		Catalog:            f.Catalog,
		Stroppy:            f.Stroppy,
		System:             f.System,
		Templates:          f.Templates,
		Runs:               f.Runs,
		Suites:             f.Suites,
		SuiteRuns:          f.SuiteRuns,
		SharedRuns:         f.SharedRuns,
		SharedSuiteRuns:    f.SharedSuiteRuns,
		Comparison:         f.Comparison,
		Baselines:          f.Baselines,
		Webhooks:           f.Webhooks,
		Quota:              f.Quota,
		BinaryCache:        f.BinaryCache,
		Admin:              f.Admin,
		BinaryAdmin:        f.BinaryAdmin,
		Agent:              f.Agent,
		IdempotencyEnabled: true,
	}))
	t.Cleanup(srv.Close)

	// Bootstrap admin + tenant.
	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "idem@e2e.test", Nickname: "idemuser"}, "Str0ng!Pass")
	require.NoError(t, err)
	pair, err := f.IAM.Login(ctx, "idem@e2e.test", "Str0ng!Pass")
	require.NoError(t, err)
	tnt, err := f.SeedTenant(ctx, "idem-tenant", u.GetId())
	require.NoError(t, err)

	cli := client.New(srv.URL,
		client.WithBearer(pair.GetAccessToken()),
		client.WithTenantHeader(tnt.GetValue()),
	)

	mkReq := func() *connect.Request[testingpb.CreateTestRunRequest] {
		r := connect.NewRequest(&testingpb.CreateTestRunRequest{
			TestRun: &testingpb.TestRun{
				Identity: &commonpb.Identity{Name: "idem-run"},
				Database: pgInlineDB(),
				Workload: inlineWorkload(),
			},
		})
		r.Header().Set("X-Idempotency-Key", "key-fixed-v1")
		return r
	}

	first, err := cli.TestRun.CreateTestRun(ctx, mkReq())
	require.NoError(t, err)
	require.NotEmpty(t, first.Msg.GetId().GetValue())

	_, err = cli.TestRun.CreateTestRun(ctx, mkReq())
	require.Error(t, err, "second call with same idempotency key must error")
	cerr, ok := err.(*connect.Error)
	require.True(t, ok)
	require.Equal(t, connect.CodeAlreadyExists, cerr.Code())
}
