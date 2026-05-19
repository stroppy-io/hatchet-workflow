//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestE2E_ApiTokens_CreateReturnsRawTokenOnce(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "tokens-tenant")

	cli := env.WithTenant(tnt.GetValue())
	resp, err := cli.ApiToken.CreateApiToken(ctx, connect.NewRequest(&iampb.CreateApiTokenRequest{
		Token: &iampb.ApiToken{Name: "ci-token"},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.GetRawToken(), "raw token must be returned on creation")
	id := resp.Msg.GetToken().GetId()

	got, err := cli.ApiToken.GetApiToken(ctx, connect.NewRequest(id))
	require.NoError(t, err)
	// Get must not leak a raw token field; the ApiToken proto has no raw_token
	// (it lives on CreateApiTokenResponse). Confirm the returned id matches.
	require.Equal(t, id.GetValue(), got.Msg.GetId().GetValue())
}

func TestE2E_ApiTokens_ListContainsToken(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "list-tokens-tenant")

	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.ApiToken.CreateApiToken(ctx, connect.NewRequest(&iampb.CreateApiTokenRequest{
		Token: &iampb.ApiToken{Name: "list-me"},
	}))
	require.NoError(t, err)
	id := created.Msg.GetToken().GetId().GetValue()

	lst, err := cli.ApiToken.ListApiTokens(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	found := false
	for _, tk := range lst.Msg.GetApiTokens() {
		if tk.GetId().GetValue() == id {
			found = true
		}
	}
	require.True(t, found, "newly created token must appear in list")
}

func TestE2E_ApiTokens_RevokeRemovesFromList(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "revoke-tenant")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.ApiToken.CreateApiToken(ctx, connect.NewRequest(&iampb.CreateApiTokenRequest{
		Token: &iampb.ApiToken{Name: "revoke-me"},
	}))
	require.NoError(t, err)
	id := created.Msg.GetToken().GetId()
	_, err = cli.ApiToken.RevokeApiToken(ctx, connect.NewRequest(id))
	require.NoError(t, err)

	lst, err := cli.ApiToken.ListApiTokens(ctx, connect.NewRequest(tnt))
	require.NoError(t, err)
	for _, tk := range lst.Msg.GetApiTokens() {
		require.NotEqual(t, id.GetValue(), tk.GetId().GetValue(), "revoked token must not appear in list")
	}
}

func TestE2E_ApiTokens_UpdateExpiry(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "expiry-tenant")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.ApiToken.CreateApiToken(ctx, connect.NewRequest(&iampb.CreateApiTokenRequest{
		Token: &iampb.ApiToken{Name: "expiry-token"},
	}))
	require.NoError(t, err)
	id := created.Msg.GetToken().GetId()

	future := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	upd, err := cli.ApiToken.UpdateApiTokenExpiry(ctx, connect.NewRequest(&iampb.UpdateApiTokenExpiryRequest{
		Id: id, ExpiresAt: timestamppb.New(future),
	}))
	require.NoError(t, err)
	require.NotNil(t, upd.Msg.GetExpiresAt())
	require.WithinDuration(t, future, upd.Msg.GetExpiresAt().AsTime(), time.Second)
}
