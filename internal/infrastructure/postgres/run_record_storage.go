package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// RunRecordStorage persists types.RunRecord values in the existing `runs`
// table (the snapshot column stores json.Marshal(rec)) and comparison
// baselines in the `baselines` table.
type RunRecordStorage struct {
	pool *pgxpool.Pool
}

func NewRunRecordStorage(pool *pgxpool.Pool) *RunRecordStorage {
	return &RunRecordStorage{pool: pool}
}

func (s *RunRecordStorage) Create(ctx context.Context, rec types.RunRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("run record storage: marshal: %w", err)
	}
	if rec.CreatedAt.IsZero() {
		_, err = s.pool.Exec(ctx,
			`INSERT INTO runs (id, tenant_id, snapshot, created_at, updated_at)
             VALUES ($1, $2, $3, NOW(), NOW())
             ON CONFLICT(id, tenant_id) DO UPDATE SET snapshot = excluded.snapshot, updated_at = NOW()`,
			rec.ID, rec.TenantID, string(data),
		)
	} else {
		_, err = s.pool.Exec(ctx,
			`INSERT INTO runs (id, tenant_id, snapshot, created_at, updated_at)
             VALUES ($1, $2, $3, $4, NOW())
             ON CONFLICT(id, tenant_id) DO UPDATE SET snapshot = excluded.snapshot, updated_at = NOW()`,
			rec.ID, rec.TenantID, string(data), rec.CreatedAt,
		)
	}
	return err
}

func (s *RunRecordStorage) Get(ctx context.Context, tenantID, id string) (*types.RunRecord, error) {
	var data string
	err := s.pool.QueryRow(ctx,
		"SELECT snapshot FROM runs WHERE id = $1 AND tenant_id = $2", id, tenantID,
	).Scan(&data)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var rec types.RunRecord
	if err := json.Unmarshal([]byte(data), &rec); err != nil {
		return nil, fmt.Errorf("run record storage: unmarshal: %w", err)
	}
	return &rec, nil
}

func (s *RunRecordStorage) List(ctx context.Context, tenantID string) ([]types.RunRecord, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT id, snapshot FROM runs WHERE tenant_id = $1 ORDER BY created_at DESC", tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.RunRecord
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var rec types.RunRecord
		if err := json.Unmarshal([]byte(data), &rec); err != nil {
			continue
		}
		results = append(results, rec)
	}
	return results, nil
}

func (s *RunRecordStorage) Delete(ctx context.Context, tenantID, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM runs WHERE id = $1 AND tenant_id = $2", id, tenantID)
	return err
}

func (s *RunRecordStorage) SetBaseline(ctx context.Context, tenantID, name, runID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO baselines (name, tenant_id, run_id) VALUES ($1, $2, $3)
         ON CONFLICT(name, tenant_id) DO UPDATE SET run_id = excluded.run_id`,
		name, tenantID, runID,
	)
	return err
}

func (s *RunRecordStorage) GetBaseline(ctx context.Context, tenantID, name string) (string, error) {
	var runID string
	err := s.pool.QueryRow(ctx,
		"SELECT run_id FROM baselines WHERE name = $1 AND tenant_id = $2", name, tenantID,
	).Scan(&runID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return runID, err
}

func (s *RunRecordStorage) ListBaselines(ctx context.Context, tenantID string) (map[string]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT name, run_id FROM baselines WHERE tenant_id = $1", tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var name, runID string
		if err := rows.Scan(&name, &runID); err != nil {
			continue
		}
		result[name] = runID
	}
	return result, nil
}
