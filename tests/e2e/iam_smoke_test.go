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
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/testserver"
)

func TestE2E_LoginCreateTenant(t *testing.T) {
	f := fixture.NewIAM(t)
	ctx := context.Background()

	// Bootstrap admin directly via service.
	_, err := f.IAM.CreateUser(ctx, &iampb.User{Email: "root@e.com", Nickname: "root"}, "RootP@ss!")
	require.NoError(t, err)

	// Stand up an HTTP test server with the IAM handler.
	handler := testserver.ForIAM(f.IAM)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Login via Connect.
	cli := client.New(srv.URL)
	resp, err := cli.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: "root@e.com", Password: "RootP@ss!"}))
	require.NoError(t, err)
	access := resp.Msg.GetTokens().GetAccessToken()
	require.NotEmpty(t, access)

	// Create a tenant as the authenticated user.
	authed := client.New(srv.URL, client.WithBearer(access))
	tenantResp, err := authed.Tenant.CreateTenant(ctx, connect.NewRequest(&iampb.CreateTenantRequest{
		Tenant: &iampb.Tenant{Identity: &commonpb.Identity{Name: "Acme"}},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, tenantResp.Msg.GetId().GetValue())
}
