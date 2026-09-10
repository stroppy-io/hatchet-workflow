// Package repositories adapts the generated sqld queries to the domain
// ports. Every repo is built over the ctx-aware executor, so calls inside
// Tx.Do* share the transaction.
package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherex/pgtx/pkg/tx"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// ProfileRepo stores profiles.
type ProfileRepo struct {
	q *db.Queries
}

var _ profile.Repository = (*ProfileRepo)(nil)

// NewProfileRepo builds the repo.
func NewProfileRepo(database tx.DB) *ProfileRepo { return &ProfileRepo{q: db.New(database)} }

func (r *ProfileRepo) Ensure(ctx context.Context, id uuid.UUID, email string) (profile.Profile, bool, error) {
	row, err := r.q.EnsureProfile(ctx, db.EnsureProfileParams{ID: id, Email: email})
	if err != nil {
		return profile.Profile{}, false, infraf("profile: ensure: %v", err)
	}
	return profileRow(db.ProfileByIDRow{
		ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Avatar: row.Avatar, IsPlatformAdmin: row.IsPlatformAdmin,
		Preferences: row.Preferences, Notifications: row.Notifications, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), row.Created != nil && *row.Created, nil
}

func (r *ProfileRepo) Get(ctx context.Context, id uuid.UUID) (profile.Profile, error) {
	row, err := r.q.ProfileByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return profile.Profile{}, errs.NotFound("profile")
		}
		return profile.Profile{}, infraf("profile: get: %v", err)
	}
	return profileRow(row), nil
}

func (r *ProfileRepo) Update(ctx context.Context, p profile.Profile) (profile.Profile, error) {
	row, err := r.q.UpdateProfile(ctx, db.UpdateProfileParams{
		ID:            p.ID,
		DisplayName:   &p.DisplayName,
		Avatar:        &p.Avatar,
		Preferences:   profile.EncodeJSON(p.Preferences),
		Notifications: profile.EncodeJSON(p.Notifications),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return profile.Profile{}, errs.NotFound("profile")
		}
		return profile.Profile{}, infraf("profile: update: %v", err)
	}
	return profileRow(db.ProfileByIDRow(row)), nil
}

func (r *ProfileRepo) SetEmail(ctx context.Context, id uuid.UUID, email string) error {
	if _, err := r.q.SetProfileEmail(ctx, db.SetProfileEmailParams{ID: id, Email: email}); err != nil {
		return infraf("profile: set email: %v", err)
	}
	return nil
}

func (r *ProfileRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.q.DeleteProfile(ctx, id); err != nil {
		return infraf("profile: delete: %v", err)
	}
	return nil
}

// profileRow maps the SELECT list; every profile query returns the same
// columns, so the other row types convert to ProfileByIDRow.
func profileRow(row db.ProfileByIDRow) profile.Profile {
	p := profile.Profile{
		ID:              row.ID,
		Email:           row.Email,
		DisplayName:     row.DisplayName,
		Avatar:          row.Avatar,
		IsPlatformAdmin: row.IsPlatformAdmin,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	_ = json.Unmarshal(row.Preferences, &p.Preferences)     //nolint:errcheck // stored by us; malformed = defaults
	_ = json.Unmarshal(row.Notifications, &p.Notifications) //nolint:errcheck // same
	return p
}
