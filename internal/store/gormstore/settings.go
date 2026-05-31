package gormstore

import (
	"context"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/system_settings"
)

// platformSettingsID is the fixed primary key of the singleton settings row.
const platformSettingsID = "platform"

// settingsRow holds the single control-plane PlatformSettings as a protojson
// blob. Exactly one row exists (id = platformSettingsID).
type settingsRow struct {
	ID        string `gorm:"primaryKey"`
	UpdatedAt time.Time
	Data      datatypes.JSON
}

func (settingsRow) TableName() string { return "platform_settings" }

// SettingsRepo backs system_settings.PlatformSettingsRepo: the singleton
// control-plane settings (admin-set, includes the agent-facing ServerAddr).
type SettingsRepo struct{ db *gorm.DB }

var _ system_settings.PlatformSettingsRepo = (*SettingsRepo)(nil)

// Settings returns the singleton platform settings repo.
func (s *Store) Settings() *SettingsRepo { return &SettingsRepo{db: s.db} }

// Get returns the current settings. A brand-new install (no row yet) returns
// derrors.ErrNotFound, which callers treat as "defaults".
func (r *SettingsRepo) Get(ctx context.Context) (*api.PlatformSettings, error) {
	var row settingsRow
	if err := r.db.WithContext(ctx).First(&row, "id = ?", platformSettingsID).Error; err != nil {
		return nil, translateGormErr("platform_settings", err)
	}
	out := &api.PlatformSettings{}
	if err := unmarshalOpts.Unmarshal(row.Data, out); err != nil {
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
	row := settingsRow{ID: platformSettingsID, UpdatedAt: time.Now(), Data: data}
	return r.db.WithContext(ctx).Save(&row).Error
}
