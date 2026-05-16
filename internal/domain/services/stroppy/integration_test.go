package stroppy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestListStroppyVersionsCached(t *testing.T) {
	releases := `[
		{"tag_name":"v4.1.0","prerelease":false,"published_at":"2026-04-01T10:00:00Z","html_url":"https://example.com/rel/v4.1.0","assets":[{"browser_download_url":"https://example.com/a"}]},
		{"tag_name":"v4.0.0","prerelease":false,"published_at":"2026-03-01T10:00:00Z","html_url":"https://example.com/rel/v4.0.0","assets":[]}
	]`
	f := fixture.NewStroppy(t, releases, "[]")

	list, err := f.Stroppy.ListStroppyVersions(context.Background())
	require.NoError(t, err)
	require.Len(t, list.GetVersions(), 2)
	require.Equal(t, "v4.1.0", list.GetVersions()[0].GetTag())

	// Close the stub server — subsequent calls must be served from cache.
	f.GH.Close()
	list2, err := f.Stroppy.ListStroppyVersions(context.Background())
	require.NoError(t, err)
	require.Len(t, list2.GetVersions(), 2)
}
