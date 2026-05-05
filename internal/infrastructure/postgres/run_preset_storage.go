package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunPreset is a saved template of run parameters (workload + infra knobs).
// Distinct from `presets` (DB topology only). The Config blob is the run
// config payload minus identity fields (id/name/description); the API layer
// strips those on save and re-injects fresh ones on load.
type RunPreset struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	DBKind      string
	Config      json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RunPresetStorage struct {
	pool *pgxpool.Pool
}

func NewRunPresetStorage(pool *pgxpool.Pool) *RunPresetStorage {
	return &RunPresetStorage{pool: pool}
}

func (s *RunPresetStorage) Create(ctx context.Context, p RunPreset) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO run_presets (id, tenant_id, name, description, db_kind, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
		p.ID, p.TenantID, p.Name, p.Description, p.DBKind, string(p.Config),
	)
	return err
}

func (s *RunPresetStorage) Update(ctx context.Context, p RunPreset) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE run_presets SET name = $3, description = $4, config = $5, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2`,
		p.ID, p.TenantID, p.Name, p.Description, string(p.Config),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("run preset not found")
	}
	return nil
}

func (s *RunPresetStorage) Get(ctx context.Context, tenantID, id string) (*RunPreset, error) {
	var p RunPreset
	var configStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, db_kind, config, created_at, updated_at
		FROM run_presets WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.DBKind, &configStr, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Config = json.RawMessage(configStr)
	return &p, nil
}

func (s *RunPresetStorage) List(ctx context.Context, tenantID, dbKind string) ([]RunPreset, error) {
	var rows pgx.Rows
	var err error
	if dbKind != "" {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, description, db_kind, config, created_at, updated_at
			FROM run_presets WHERE tenant_id = $1 AND db_kind = $2 ORDER BY name`,
			tenantID, dbKind,
		)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id, tenant_id, name, description, db_kind, config, created_at, updated_at
			FROM run_presets WHERE tenant_id = $1 ORDER BY name`,
			tenantID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RunPreset
	for rows.Next() {
		var p RunPreset
		var configStr string
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &p.DBKind, &configStr, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Config = json.RawMessage(configStr)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *RunPresetStorage) Delete(ctx context.Context, tenantID, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM run_presets WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	return err
}
