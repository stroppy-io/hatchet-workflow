package workflows

import (
	"context"
	"errors"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
)

// fakeSettings is a stand-in settingsReader.
type fakeSettings struct {
	settings *api.PlatformSettings
	err      error
}

func (f fakeSettings) Get(context.Context) (*api.PlatformSettings, error) {
	return f.settings, f.err
}

func TestResolveServerAddr(t *testing.T) {
	const dflt = "http://host.docker.internal:8080"

	cases := []struct {
		name string
		sa   *ServerActivities
		want string
	}{
		{
			name: "no settings reader -> default",
			sa:   &ServerActivities{DefaultServerAddr: dflt},
			want: dflt,
		},
		{
			name: "settings error (no row yet) -> default",
			sa:   &ServerActivities{DefaultServerAddr: dflt, Settings: fakeSettings{err: errors.New("not found")}},
			want: dflt,
		},
		{
			name: "settings empty ServerAddr -> default",
			sa:   &ServerActivities{DefaultServerAddr: dflt, Settings: fakeSettings{settings: &api.PlatformSettings{}}},
			want: dflt,
		},
		{
			name: "admin-set ServerAddr wins",
			sa: &ServerActivities{
				DefaultServerAddr: dflt,
				Settings:          fakeSettings{settings: &api.PlatformSettings{ServerAddr: "https://ctrl.example.com"}},
			},
			want: "https://ctrl.example.com",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sa.resolveServerAddr(context.Background()); got != c.want {
				t.Fatalf("resolveServerAddr = %q, want %q", got, c.want)
			}
		})
	}
}
