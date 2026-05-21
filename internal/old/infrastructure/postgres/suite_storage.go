package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuitePolicy controls how the scheduler admits step jobs of a launched
// batch. Sequential is the default (blocks N until N-1 is terminal);
// parallel admits up to MaxParallel jobs from the same batch concurrently.
type SuitePolicy struct {
	Mode           string `json:"mode"`             // "sequential" | "parallel"
	MaxParallel    int    `json:"max_parallel"`     // parallel only; 0 = unlimited
	OnStepFail     string `json:"on_step_fail"`     // "continue" | "stop"
	StepTimeoutMin int    `json:"step_timeout_min"` // 0 = no timeout
}

func DefaultSuitePolicy() SuitePolicy {
	return SuitePolicy{Mode: "sequential", MaxParallel: 1, OnStepFail: "continue"}
}

// ComparisonConfig declares how to pair the batch produced by this suite
// against an earlier one when the API is asked for a diff. Compute itself
// is on-demand (metrics.Compare per (db_preset_id, suite_item_id) pair);
// this struct only stores user intent.
type ComparisonConfig struct {
	Strategy        string  `json:"strategy,omitempty"` // "previous" | "first" | "fixed" | ""
	BaselineBatchID string  `json:"baseline_batch_id,omitempty"`
	ThresholdPct    float64 `json:"threshold_pct,omitempty"`
}

// Suite is a (db_preset × workload) matrix. A single launch produces
// len(DBPresetIDs) * len(items) job_runs — one per matrix cell. Shared
// infrastructure (provider, network, stroppy machine) lives on the suite
// itself so adding a workload doesn't need to repeat it for every preset.
type Suite struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Policy      SuitePolicy
	Comparison  ComparisonConfig

	// Matrix axes.
	DBPresetIDs []string // database axis — references presets.id
	// Items axis lives in suite_items table (loaded separately).

	// Shared infrastructure across every matrix cell.
	Provider       string
	PlatformID     string
	Network        json.RawMessage // shape: {cidr, zone}
	StroppyMachine json.RawMessage // shape: MachineSpec; empty = server defaults

	// Scheduling. CronExpr empty = manual-only suite.
	CronExpr         string
	Timezone         string
	Enabled          bool
	NextFireAt       *time.Time
	LastFireAt       *time.Time
	LastBatchID      string
	ConcurrentPolicy string // "forbid" | "allow"
	CatchupMode      string // "skip" | "once"

	// Cron lease for multi-server safety.
	LockOwner     string
	LockExpiresAt *time.Time

	RetentionRuns int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// SuiteItem is one workload definition (the "items axis"). The Workload
// blob is a stroppy-only config (StroppyConfig in the public API). Combined
// with one of the suite's DBPresetIDs at launch time it forms a complete
// RunConfig.
type SuiteItem struct {
	ID        string
	SuiteID   string
	TenantID  string
	Position  int
	Name      string
	Workload  json.RawMessage
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SuiteStorage struct {
	pool *pgxpool.Pool
}

func NewSuiteStorage(pool *pgxpool.Pool) *SuiteStorage {
	return &SuiteStorage{pool: pool}
}

// --- Suite CRUD ---

func (s *SuiteStorage) Create(ctx context.Context, su Suite) error {
	if su.Policy.Mode == "" {
		su.Policy = DefaultSuitePolicy()
	}
	if su.Timezone == "" {
		su.Timezone = "UTC"
	}
	if su.ConcurrentPolicy == "" {
		su.ConcurrentPolicy = "forbid"
	}
	if su.CatchupMode == "" {
		su.CatchupMode = "skip"
	}
	if su.RetentionRuns == 0 {
		su.RetentionRuns = 30
	}
	if su.Provider == "" {
		su.Provider = "docker"
	}
	if len(su.Network) == 0 {
		su.Network = json.RawMessage(`{"cidr":"10.10.0.0/24"}`)
	}
	if len(su.StroppyMachine) == 0 {
		su.StroppyMachine = json.RawMessage(`{}`)
	}
	policyJSON, _ := json.Marshal(su.Policy)
	cmpJSON, _ := json.Marshal(su.Comparison)
	if su.DBPresetIDs == nil {
		su.DBPresetIDs = []string{}
	}
	dbsJSON, _ := json.Marshal(su.DBPresetIDs)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO suites (
			id, tenant_id, name, description,
			execution_policy, comparison_config,
			cron_expr, timezone, enabled,
			next_fire_at, concurrent_policy, catchup_mode,
			retention_runs,
			db_preset_ids, provider, platform_id, network, stroppy_machine,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4, $5,$6, $7,$8,$9, $10,$11,$12, $13,
		          $14,$15,$16,$17,$18, NOW(), NOW())`,
		su.ID, su.TenantID, su.Name, su.Description,
		string(policyJSON), string(cmpJSON),
		nullStr(su.CronExpr), su.Timezone, su.Enabled,
		su.NextFireAt, su.ConcurrentPolicy, su.CatchupMode,
		su.RetentionRuns,
		string(dbsJSON), su.Provider, nullStr(su.PlatformID),
		string(su.Network), string(su.StroppyMachine),
	)
	return err
}

func (s *SuiteStorage) Update(ctx context.Context, su Suite) error {
	if su.Policy.Mode == "" {
		su.Policy = DefaultSuitePolicy()
	}
	policyJSON, _ := json.Marshal(su.Policy)
	cmpJSON, _ := json.Marshal(su.Comparison)
	if su.DBPresetIDs == nil {
		su.DBPresetIDs = []string{}
	}
	dbsJSON, _ := json.Marshal(su.DBPresetIDs)
	if len(su.Network) == 0 {
		su.Network = json.RawMessage(`{"cidr":"10.10.0.0/24"}`)
	}
	if len(su.StroppyMachine) == 0 {
		su.StroppyMachine = json.RawMessage(`{}`)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE suites
		   SET name=$3, description=$4,
		       execution_policy=$5, comparison_config=$6,
		       cron_expr=$7, timezone=$8, enabled=$9,
		       next_fire_at=$10, concurrent_policy=$11, catchup_mode=$12,
		       retention_runs=$13,
		       db_preset_ids=$14, provider=$15, platform_id=$16,
		       network=$17, stroppy_machine=$18,
		       updated_at=NOW()
		 WHERE id=$1 AND tenant_id=$2`,
		su.ID, su.TenantID, su.Name, su.Description,
		string(policyJSON), string(cmpJSON),
		nullStr(su.CronExpr), su.Timezone, su.Enabled,
		su.NextFireAt, su.ConcurrentPolicy, su.CatchupMode,
		su.RetentionRuns,
		string(dbsJSON), su.Provider, nullStr(su.PlatformID),
		string(su.Network), string(su.StroppyMachine),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("suite not found")
	}
	return nil
}

const suiteSelectCols = `id, tenant_id, name, description,
	COALESCE(execution_policy, '{}'),
	COALESCE(comparison_config, '{}'),
	COALESCE(cron_expr, ''), timezone, enabled,
	next_fire_at, last_fire_at, COALESCE(last_batch_id, ''),
	concurrent_policy, catchup_mode,
	COALESCE(lock_owner, ''), lock_expires_at,
	retention_runs,
	COALESCE(db_preset_ids, '[]'),
	provider, COALESCE(platform_id, ''),
	COALESCE(network, '{}'), COALESCE(stroppy_machine, '{}'),
	created_at, updated_at`

func scanSuite(row pgx.Row) (*Suite, error) {
	var (
		su        Suite
		policyStr string
		cmpStr    string
		dbsStr    string
		netStr    string
		machStr   string
	)
	err := row.Scan(
		&su.ID, &su.TenantID, &su.Name, &su.Description,
		&policyStr, &cmpStr,
		&su.CronExpr, &su.Timezone, &su.Enabled,
		&su.NextFireAt, &su.LastFireAt, &su.LastBatchID,
		&su.ConcurrentPolicy, &su.CatchupMode,
		&su.LockOwner, &su.LockExpiresAt,
		&su.RetentionRuns,
		&dbsStr,
		&su.Provider, &su.PlatformID,
		&netStr, &machStr,
		&su.CreatedAt, &su.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if policyStr != "" && policyStr != "{}" {
		_ = json.Unmarshal([]byte(policyStr), &su.Policy)
	}
	if su.Policy.Mode == "" {
		su.Policy = DefaultSuitePolicy()
	}
	if cmpStr != "" && cmpStr != "{}" {
		_ = json.Unmarshal([]byte(cmpStr), &su.Comparison)
	}
	if dbsStr != "" {
		_ = json.Unmarshal([]byte(dbsStr), &su.DBPresetIDs)
	}
	if su.DBPresetIDs == nil {
		su.DBPresetIDs = []string{}
	}
	su.Network = json.RawMessage(netStr)
	su.StroppyMachine = json.RawMessage(machStr)
	return &su, nil
}

func (s *SuiteStorage) Get(ctx context.Context, tenantID, id string) (*Suite, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+suiteSelectCols+` FROM suites WHERE id=$1 AND tenant_id=$2`,
		id, tenantID)
	return scanSuite(row)
}

func (s *SuiteStorage) List(ctx context.Context, tenantID string) ([]Suite, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+suiteSelectCols+` FROM suites WHERE tenant_id=$1 ORDER BY name`,
		tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Suite
	for rows.Next() {
		su, err := scanSuite(rows)
		if err != nil {
			return nil, err
		}
		if su != nil {
			out = append(out, *su)
		}
	}
	return out, rows.Err()
}

func (s *SuiteStorage) Delete(ctx context.Context, tenantID, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM suites WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	return err
}

// --- Suite items (workloads axis) CRUD ---

const suiteItemCols = `id, suite_id, tenant_id, position, name,
	workload, enabled, created_at, updated_at`

func scanSuiteItem(row pgx.Row) (*SuiteItem, error) {
	var (
		it    SuiteItem
		wlStr string
	)
	err := row.Scan(
		&it.ID, &it.SuiteID, &it.TenantID, &it.Position, &it.Name,
		&wlStr, &it.Enabled, &it.CreatedAt, &it.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	it.Workload = json.RawMessage(wlStr)
	return &it, nil
}

func (s *SuiteStorage) ListItems(ctx context.Context, tenantID, suiteID string) ([]SuiteItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+suiteItemCols+` FROM suite_items
		   WHERE tenant_id=$1 AND suite_id=$2 ORDER BY position ASC`,
		tenantID, suiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SuiteItem
	for rows.Next() {
		it, err := scanSuiteItem(rows)
		if err != nil {
			return nil, err
		}
		if it != nil {
			out = append(out, *it)
		}
	}
	return out, rows.Err()
}

func (s *SuiteStorage) GetItem(ctx context.Context, tenantID, itemID string) (*SuiteItem, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+suiteItemCols+` FROM suite_items WHERE tenant_id=$1 AND id=$2`,
		tenantID, itemID)
	return scanSuiteItem(row)
}

func (s *SuiteStorage) CreateItem(ctx context.Context, it SuiteItem) error {
	if len(it.Workload) == 0 {
		return errors.New("suite item: workload required")
	}
	if it.Position < 0 {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO suite_items (`+suiteItemCols+`)
			SELECT $1,$2,$3,
			       COALESCE((SELECT MAX(position)+1 FROM suite_items WHERE suite_id=$2), 0),
			       $4,$5,$6,NOW(),NOW()`,
			it.ID, it.SuiteID, it.TenantID, it.Name, string(it.Workload), it.Enabled)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO suite_items (id, suite_id, tenant_id, position, name,
		                         workload, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW())`,
		it.ID, it.SuiteID, it.TenantID, it.Position, it.Name,
		string(it.Workload), it.Enabled)
	return err
}

func (s *SuiteStorage) UpdateItem(ctx context.Context, it SuiteItem) error {
	if len(it.Workload) == 0 {
		return errors.New("suite item: workload required")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE suite_items
		   SET name=$3, workload=$4, enabled=$5, updated_at=NOW()
		 WHERE tenant_id=$1 AND id=$2`,
		it.TenantID, it.ID, it.Name, string(it.Workload), it.Enabled)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("suite item not found")
	}
	return nil
}

func (s *SuiteStorage) DeleteItem(ctx context.Context, tenantID, itemID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM suite_items WHERE tenant_id=$1 AND id=$2`,
		tenantID, itemID)
	return err
}

// ReorderItems applies a full new ordering (item_id → position).
// Two-phase shift through negative positions so the UNIQUE(suite_id, position)
// index never sees collisions mid-update.
func (s *SuiteStorage) ReorderItems(ctx context.Context, tenantID, suiteID string, order map[string]int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for id := range order {
		if _, err := tx.Exec(ctx, `
			UPDATE suite_items SET position = -position - 1, updated_at=NOW()
			 WHERE tenant_id=$1 AND suite_id=$2 AND id=$3`,
			tenantID, suiteID, id); err != nil {
			return err
		}
	}
	for id, pos := range order {
		if _, err := tx.Exec(ctx, `
			UPDATE suite_items SET position=$4, updated_at=NOW()
			 WHERE tenant_id=$1 AND suite_id=$2 AND id=$3`,
			tenantID, suiteID, id, pos); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// --- Cron lease ---

func (s *SuiteStorage) DueCron(ctx context.Context, now time.Time, limit int) ([]Suite, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+suiteSelectCols+` FROM suites
		  WHERE enabled = TRUE
		    AND cron_expr IS NOT NULL
		    AND next_fire_at IS NOT NULL
		    AND next_fire_at <= $1
		    AND (lock_expires_at IS NULL OR lock_expires_at < $1)
		  ORDER BY next_fire_at ASC
		  LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Suite
	for rows.Next() {
		su, err := scanSuite(rows)
		if err != nil {
			return nil, err
		}
		if su != nil {
			out = append(out, *su)
		}
	}
	return out, rows.Err()
}

func (s *SuiteStorage) AcquireCronLease(ctx context.Context, suiteID, owner string, fireAt time.Time, ttl time.Duration) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE suites
		   SET lock_owner=$2, lock_expires_at=$3
		 WHERE id=$1
		   AND (lock_expires_at IS NULL OR lock_expires_at < NOW())
		   AND next_fire_at IS NOT NULL
		   AND next_fire_at <= $4`,
		suiteID, owner, time.Now().Add(ttl), fireAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *SuiteStorage) ReleaseCronLease(ctx context.Context, suiteID, owner string, lastFireAt, nextFireAt time.Time, batchID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE suites
		   SET lock_owner=NULL, lock_expires_at=NULL,
		       last_fire_at=$3, next_fire_at=$4, last_batch_id=$5,
		       updated_at=NOW()
		 WHERE id=$1 AND (lock_owner=$2 OR lock_owner IS NULL)`,
		suiteID, owner, lastFireAt, nextFireAt, nullStr(batchID))
	return err
}

// ItemCounts returns suite_id → enabled-item count for one tenant in a
// single round trip. Used by listSuites to render the "Workloads" column
// without N+1 SELECTs against suite_items.
func (s *SuiteStorage) ItemCounts(ctx context.Context, tenantID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT suite_id, COUNT(*)::int FROM suite_items
		 WHERE tenant_id=$1 GROUP BY suite_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var sid string
		var n int
		if err := rows.Scan(&sid, &n); err != nil {
			return nil, err
		}
		out[sid] = n
	}
	return out, rows.Err()
}

// MarkLaunched stamps last_fire_at + last_batch_id for non-cron launches.
// Cron path uses ReleaseCronLease which also clears the lease; manual /
// API launches go through here so the suites list shows fresh "last
// batch" info instead of staying blank until the next cron fire.
func (s *SuiteStorage) MarkLaunched(ctx context.Context, tenantID, suiteID, batchID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE suites
		   SET last_fire_at=NOW(), last_batch_id=$3, updated_at=NOW()
		 WHERE id=$1 AND tenant_id=$2`,
		suiteID, tenantID, nullStr(batchID))
	return err
}

func (s *SuiteStorage) SetNextFire(ctx context.Context, tenantID, suiteID string, next *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE suites SET next_fire_at=$3, updated_at=NOW()
		   WHERE id=$1 AND tenant_id=$2`,
		suiteID, tenantID, next)
	return err
}
