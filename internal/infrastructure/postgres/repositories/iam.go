package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// IAMRepo is the IAM side-state: revoked sessions and webhook dedupe.
type IAMRepo struct {
	q *db.Queries
}

// NewIAMRepo builds the repo.
func NewIAMRepo(database tx.DB) *IAMRepo { return &IAMRepo{q: db.New(database)} }

// DenySession records a revoked session until its access tokens expire.
func (r *IAMRepo) DenySession(ctx context.Context, sessionID string, userID uuid.UUID, until time.Time) error {
	if err := r.q.DenySession(ctx, db.DenySessionParams{SessionID: sessionID, UserID: userID, ExpiresAt: until}); err != nil {
		return infraf("iam: deny session: %v", err)
	}
	return nil
}

// SessionDenied reports whether the session was revoked.
func (r *IAMRepo) SessionDenied(ctx context.Context, sessionID string) (bool, error) {
	if _, err := r.q.DeniedSession(ctx, sessionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, infraf("iam: session denied: %v", err)
	}
	return true, nil
}

// RecordWebhookEvent returns false on a duplicate delivery.
func (r *IAMRepo) RecordWebhookEvent(ctx context.Context, id string) (bool, error) {
	n, err := r.q.RecordWebhookEvent(ctx, id)
	if err != nil {
		return false, infraf("iam: record webhook: %v", err)
	}
	return n > 0, nil
}

// Purge drops expired denylist rows and old webhook ids.
func (r *IAMRepo) Purge(ctx context.Context, webhooksBefore time.Time) error {
	if _, err := r.q.PurgeDenylist(ctx); err != nil {
		return infraf("iam: purge denylist: %v", err)
	}
	if _, err := r.q.PurgeWebhookEvents(ctx, webhooksBefore); err != nil {
		return infraf("iam: purge webhooks: %v", err)
	}
	return nil
}
