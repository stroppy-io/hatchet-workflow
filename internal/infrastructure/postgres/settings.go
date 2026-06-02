package postgres

import (
	"context"

	dbgen "github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/system_settings"
)

// platformSettingsID is the fixed primary key of the singleton settings row.
const platformSettingsID = "platform"

// SettingsRepo backs system_settings.PlatformSettingsRepo: the singleton
// control-plane settings (admin-set, includes the agent-facing ServerAddr).
type SettingsRepo struct{ db *DB }

var _ system_settings.PlatformSettingsRepo = (*SettingsRepo)(nil)

// Settings returns the singleton platform settings repo.
func (s *Store) Settings() *SettingsRepo { return &SettingsRepo{db: s.db} }

// Get returns the current settings. A brand-new install (no row yet) returns
// derrors.ErrNotFound, which callers treat as "defaults".
func (r *SettingsRepo) Get(ctx context.Context) (*api.PlatformSettings, error) {
	row, err := r.db.q().GetPlatformSettings(ctx, platformSettingsID)
	if err != nil {
		return nil, translatePgErr("platform_settings", err)
	}
	out := &api.PlatformSettings{}
	if err := unmarshal(row.Data, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Set replaces the singleton wholesale (upsert by primary key).
func (r *SettingsRepo) Set(ctx context.Context, settings *api.PlatformSettings) error {
	data, err := marshal(settings)
	if err != nil {
		return err
	}
	return r.db.q().UpsertPlatformSettings(ctx, dbgen.UpsertPlatformSettingsParams{
		ID:   platformSettingsID,
		Data: data,
	})
}
