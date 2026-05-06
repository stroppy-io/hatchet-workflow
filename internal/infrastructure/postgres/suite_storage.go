package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuiteItem is one entry in a suite — a reference to a run-preset plus an
// optional partial RunConfig override applied at launch time. Overrides is
// stored as raw JSON; the executor merges it on top of the resolved
// run-preset config when starting the run.
type SuiteItem struct {
	RunPresetID string          `json:"run_preset_id"`
	Overrides   json.RawMessage `json:"overrides,omitempty"`
}

// SuitePolicy controls how the scheduler admits step jobs of a launched
// batch. Sequential is the default (blocks N until N-1 is terminal);
// parallel admits up to MaxParallel jobs from the same batch concurrently.
type SuitePolicy struct {
	Mode           string `json:"mode"`             // "sequential" | "parallel"
	MaxParallel    int    `json:"max_parallel"`     // parallel only; 0 = unlimited
	OnStepFail     string `json:"on_step_fail"`     // "continue" | "stop"
	StepTimeoutMin int    `json:"step_timeout_min"` // 0 = no timeout
}

// DefaultSuitePolicy returns the policy used for suites that haven't set
// one explicitly. Mirrors the migration's column default.
func DefaultSuitePolicy() SuitePolicy {
	return SuitePolicy{Mode: "sequential", MaxParallel: 1, OnStepFail: "continue"}
}

// Suite is an ordered list of run-preset launches that can be executed as a
// batch and re-run later with overrides.
type Suite struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Items       []SuiteItem
	Policy      SuitePolicy
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SuiteRunRow links a suite execution batch to one produced run.
type SuiteRunRow struct {
	SuiteID   string
	BatchID   string
	RunID     string
	TenantID  string
	Position  int
	CreatedAt time.Time
}

type SuiteStorage struct {
	pool *pgxpool.Pool
}

func NewSuiteStorage(pool *pgxpool.Pool) *SuiteStorage {
	return &SuiteStorage{pool: pool}
}

func (s *SuiteStorage) Create(ctx context.Context, su Suite) error {
	itemsJSON, err := json.Marshal(su.Items)
	if err != nil {
		return err
	}
	if su.Policy.Mode == "" {
		su.Policy = DefaultSuitePolicy()
	}
	policyJSON, err := json.Marshal(su.Policy)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO suites (id, tenant_id, name, description, items, execution_policy, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`,
		su.ID, su.TenantID, su.Name, su.Description, string(itemsJSON), string(policyJSON),
	)
	return err
}

func (s *SuiteStorage) Update(ctx context.Context, su Suite) error {
	itemsJSON, err := json.Marshal(su.Items)
	if err != nil {
		return err
	}
	if su.Policy.Mode == "" {
		su.Policy = DefaultSuitePolicy()
	}
	policyJSON, err := json.Marshal(su.Policy)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE suites SET name = $3, description = $4, items = $5, execution_policy = $6, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2`,
		su.ID, su.TenantID, su.Name, su.Description, string(itemsJSON), string(policyJSON),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("suite not found")
	}
	return nil
}

func (s *SuiteStorage) Get(ctx context.Context, tenantID, id string) (*Suite, error) {
	var su Suite
	var itemsStr, policyStr string
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, items,
		       COALESCE(execution_policy, '{}'),
		       created_at, updated_at
		FROM suites WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&su.ID, &su.TenantID, &su.Name, &su.Description, &itemsStr, &policyStr, &su.CreatedAt, &su.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(itemsStr), &su.Items); err != nil {
		return nil, err
	}
	if policyStr != "" && policyStr != "{}" {
		_ = json.Unmarshal([]byte(policyStr), &su.Policy)
	}
	if su.Policy.Mode == "" {
		su.Policy = DefaultSuitePolicy()
	}
	return &su, nil
}

func (s *SuiteStorage) List(ctx context.Context, tenantID string) ([]Suite, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, items,
		       COALESCE(execution_policy, '{}'),
		       created_at, updated_at
		FROM suites WHERE tenant_id = $1 ORDER BY name`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Suite
	for rows.Next() {
		var su Suite
		var itemsStr, policyStr string
		if err := rows.Scan(&su.ID, &su.TenantID, &su.Name, &su.Description, &itemsStr, &policyStr, &su.CreatedAt, &su.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(itemsStr), &su.Items); err != nil {
			continue
		}
		if policyStr != "" && policyStr != "{}" {
			_ = json.Unmarshal([]byte(policyStr), &su.Policy)
		}
		if su.Policy.Mode == "" {
			su.Policy = DefaultSuitePolicy()
		}
		out = append(out, su)
	}
	return out, rows.Err()
}

func (s *SuiteStorage) Delete(ctx context.Context, tenantID, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM suites WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	return err
}

func (s *SuiteStorage) RecordRun(ctx context.Context, row SuiteRunRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO suite_runs (suite_id, batch_id, run_id, tenant_id, position, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT DO NOTHING`,
		row.SuiteID, row.BatchID, row.RunID, row.TenantID, row.Position,
	)
	return err
}

// ListRuns returns all suite_runs rows for a suite, newest batch first.
func (s *SuiteStorage) ListRuns(ctx context.Context, tenantID, suiteID string) ([]SuiteRunRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT suite_id, batch_id, run_id, tenant_id, position, created_at
		FROM suite_runs WHERE tenant_id = $1 AND suite_id = $2
		ORDER BY created_at DESC, position ASC`,
		tenantID, suiteID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SuiteRunRow
	for rows.Next() {
		var r SuiteRunRow
		if err := rows.Scan(&r.SuiteID, &r.BatchID, &r.RunID, &r.TenantID, &r.Position, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
