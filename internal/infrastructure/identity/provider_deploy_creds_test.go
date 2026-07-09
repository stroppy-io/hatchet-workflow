package identity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
)

type fakeSecretStore struct {
	values map[string]map[string]string // namespace -> key -> value
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{values: map[string]map[string]string{}}
}

func (f *fakeSecretStore) Put(_ context.Context, namespace, key, value string) error {
	if f.values[namespace] == nil {
		f.values[namespace] = map[string]string{}
	}
	f.values[namespace][key] = value
	return nil
}

func (f *fakeSecretStore) Fetch(_ context.Context, namespace, key string) (string, error) {
	v, ok := f.values[namespace][key]
	if !ok {
		return "", derrors.ErrNotFound
	}
	return v, nil
}

func (f *fakeSecretStore) Remove(_ context.Context, namespace, key string) error {
	delete(f.values[namespace], key)
	return nil
}

func TestProviderDeployCreds_SetGet_RoundTripsPerTenant(t *testing.T) {
	store := newFakeSecretStore()
	creds, err := NewProviderDeployCreds(store, Config{SecretEncryptionKey: make([]byte, 32)})
	require.NoError(t, err)

	require.NoError(t, creds.Set(context.Background(), "tenant-a", map[string]string{"YC_TOKEN": "token-a"}))
	require.NoError(t, creds.Set(context.Background(), "tenant-b", map[string]string{"YC_TOKEN": "token-b"}))

	gotA, err := creds.Get(context.Background(), "tenant-a")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"YC_TOKEN": "token-a"}, gotA)

	gotB, err := creds.Get(context.Background(), "tenant-b")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"YC_TOKEN": "token-b"}, gotB, "tenant-b must never see tenant-a's token")

	// Sealed at rest: raw store value must not contain the plaintext token.
	raw, err := store.Fetch(context.Background(), nsProviderDeployCred, "tenant-a")
	require.NoError(t, err)
	require.NotContains(t, raw, "token-a")
}

func TestProviderDeployCreds_Get_UnknownTenant_ReturnsNotFound(t *testing.T) {
	store := newFakeSecretStore()
	creds, err := NewProviderDeployCreds(store, Config{SecretEncryptionKey: make([]byte, 32)})
	require.NoError(t, err)

	_, err = creds.Get(context.Background(), "no-such-tenant")
	require.ErrorIs(t, err, derrors.ErrNotFound,
		"no configured creds must be an explicit not-found error, never a silent fallback to process env")
}
