package yandex

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

func TestSerialize(t *testing.T) {
	cloudInit := &crossplane.CloudInit{
		Users: []*crossplane.User{
			{
				Name:              "testuser",
				SshAuthorizedKeys: []string{"ssh-rsa key1"},
			},
		},
	}
	data, err := GenerateCloudInit(cloudInit)
	require.NoError(t, err)
	t.Log(string(data))
}
