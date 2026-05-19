//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

const stubReleasesJSON = `[
  {"tag_name":"v4.1.0","prerelease":false,"published_at":"2025-01-01T00:00:00Z","html_url":"http://example.invalid/v4.1.0","assets":[]},
  {"tag_name":"v4.2.0-rc.1","prerelease":true,"published_at":"2025-02-01T00:00:00Z","html_url":"http://example.invalid/v4.2.0-rc.1","assets":[]}
]`

const stubCommitsJSON = `[
  {"sha":"abcdef1234567890abcdef1234567890abcdef12","commit":{"message":"feat: thing","author":{"name":"alice","date":"2025-01-02T00:00:00Z"}}}
]`

func TestE2E_Stroppy_ListStroppyVersions_ParsesStub(t *testing.T) {
	env := newEnv(t, stubReleasesJSON, stubCommitsJSON)
	ctx := context.Background()
	resp, err := env.AdminClient.Stroppy.ListStroppyVersions(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.GetVersions()), 1)
}

func TestE2E_Stroppy_ListStroppyCommits_ParsesStub(t *testing.T) {
	env := newEnv(t, stubReleasesJSON, stubCommitsJSON)
	ctx := context.Background()
	resp, err := env.AdminClient.Stroppy.ListStroppyCommits(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Msg.GetCommits()), 1)
}
