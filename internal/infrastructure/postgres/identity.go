package postgres

import (
	"context"

	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/identity"
)

/*
	===== identity backing stores =====

	Three small Postgres-backed stores the identity adapters need: an opaque
	keyed secret blob (SecretStore), server-side refresh sessions
	(RefreshSessionStore) and short-lived SSO flow state (SSOStateStore). Each is
	a plain table; reads map pgx.ErrNoRows onto the domain not-found. All calls go
	through the sqld-generated query set bound to db.TxDB so they participate in
	the ambient transaction.
*/

// Secrets returns the identity.SecretStore (api-token hashes + provider secrets).
func (s *Store) Secrets() *SecretStore { return &SecretStore{db: s.db} }

// RefreshSessions returns the identity.RefreshSessionStore.
func (s *Store) RefreshSessions() *RefreshSessionStore { return &RefreshSessionStore{db: s.db} }

// SSOStates returns the identity.SSOStateStore.
func (s *Store) SSOStates() *SSOStateStore { return &SSOStateStore{db: s.db} }

/*
	===== SecretStore =====

	Opaque blob keyed by (namespace, key). A namespace separates api-token hashes
	from provider secrets so the adapters share one table without key collisions.
*/

type SecretStore struct{ db *DB }

var _ identity.SecretStore = (*SecretStore)(nil)

func (s *SecretStore) Put(ctx context.Context, namespace, key, value string) error {
	return s.db.q().UpsertIdentitySecret(ctx, dbgen.UpsertIdentitySecretParams{
		Namespace: namespace,
		Key:       key,
		Value:     value,
	})
}

func (s *SecretStore) Fetch(ctx context.Context, namespace, key string) (string, error) {
	row, err := s.db.q().GetIdentitySecret(ctx, dbgen.GetIdentitySecretParams{Namespace: namespace, Key: key})
	if err != nil {
		return "", translatePgErr("secret", err)
	}
	return row.Value, nil
}

func (s *SecretStore) Remove(ctx context.Context, namespace, key string) error {
	return s.db.q().DeleteIdentitySecret(ctx, dbgen.DeleteIdentitySecretParams{Namespace: namespace, Key: key})
}

/*
	===== RefreshSessionStore =====

	Server-side refresh-token records: single-use and revocable. Get reports an
	unknown/consumed session as not-found.
*/

type RefreshSessionStore struct{ db *DB }

var _ identity.RefreshSessionStore = (*RefreshSessionStore)(nil)

func (s *RefreshSessionStore) Create(ctx context.Context, sess identity.RefreshSession) error {
	return s.db.q().CreateIdentityRefreshSession(ctx, dbgen.CreateIdentityRefreshSessionParams{
		ID:        sess.ID,
		AccountID: sess.AccountID,
		ExpiresAt: sess.ExpiresAt,
	})
}

func (s *RefreshSessionStore) Get(ctx context.Context, id string) (identity.RefreshSession, error) {
	row, err := s.db.q().GetIdentityRefreshSession(ctx, id)
	if err != nil {
		return identity.RefreshSession{}, translatePgErr("refresh_session", err)
	}
	return identity.RefreshSession{
		ID:        row.ID,
		AccountID: row.AccountID,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

func (s *RefreshSessionStore) Delete(ctx context.Context, id string) error {
	return s.db.q().DeleteIdentityRefreshSession(ctx, id)
}

/*
	===== SSOStateStore =====

	Short-lived SSO flow state stashed between Authorize and the callback. Consume
	is single-use: it deletes and returns the row in one statement; an
	unknown/expired/consumed key is reported as not-found.
*/

type SSOStateStore struct{ db *DB }

var _ identity.SSOStateStore = (*SSOStateStore)(nil)

func (s *SSOStateStore) Save(ctx context.Context, st identity.SSOState) error {
	return s.db.q().SaveIdentitySsoState(ctx, dbgen.SaveIdentitySsoStateParams{
		State:        st.State,
		ProviderID:   st.ProviderID,
		CodeVerifier: st.CodeVerifier,
		Nonce:        st.Nonce,
		ExpiresAt:    st.ExpiresAt,
	})
}

func (s *SSOStateStore) Consume(ctx context.Context, state string) (identity.SSOState, error) {
	row, err := s.db.q().ConsumeIdentitySsoState(ctx, state)
	if err != nil {
		return identity.SSOState{}, translatePgErr("sso_state", err)
	}
	return identity.SSOState{
		State:        row.State,
		ProviderID:   row.ProviderID,
		CodeVerifier: row.CodeVerifier,
		Nonce:        row.Nonce,
		ExpiresAt:    row.ExpiresAt,
	}, nil
}
