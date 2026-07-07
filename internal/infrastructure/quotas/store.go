package quotas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

// Reserve/CommitRun/ReleaseRun's sentinel errors — recovered verbatim from
// the pre-DSL-pivot Store (git show efd9bffc^:internal/infrastructure/
// quotas/store.go) that this file's Reserve/CommitRun/ReleaseRun remodel
// restores (see reserve.go's package doc for the remodel: the INPUT is now
// []QuotaAmount computed from dslpb.MachineGroup, not []*workflowpb.
// QuotaRequestRef).
var (
	ErrSnapshotMissing = errors.New("quota snapshot missing")
	ErrSnapshotStale   = errors.New("quota snapshot stale")
	ErrInsufficient    = errors.New("insufficient quota")
)

type Store struct {
	db *postgres.DB
}

func NewStore(db *postgres.DB) *Store {
	return &Store{db: db}
}

func (s *Store) UpsertSnapshots(ctx context.Context, snapshots []Snapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	tx, err := s.db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	for _, snap := range snapshots {
		if _, err := tx.Exec(ctx, `
insert into quota_snapshots
  (tenant_id, provider, resource_type, resource_id, service, quota_name, units,
   provider_used, quota_limit, provider_available, observed_at, stale_after, raw)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,coalesce($13, '{}'::jsonb))
on conflict (tenant_id, provider, resource_type, resource_id, quota_name)
do update set
  service = excluded.service,
  units = excluded.units,
  provider_used = excluded.provider_used,
  quota_limit = excluded.quota_limit,
  provider_available = excluded.provider_available,
  observed_at = excluded.observed_at,
  stale_after = excluded.stale_after,
  raw = excluded.raw`,
			snap.TenantID,
			int32(snap.Provider),
			snap.ResourceType,
			snap.ResourceID,
			snap.Service,
			snap.QuotaName,
			snap.Units,
			snap.ProviderUsed,
			snap.Limit,
			snap.ProviderAvailable,
			snap.ObservedAt,
			snap.StaleAfter,
			snap.Raw,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ScopeNeedsRefresh(ctx context.Context, scope Scope, quotaNames []string, now time.Time) (bool, error) {
	if len(quotaNames) == 0 {
		var stale int
		err := s.db.Pool.QueryRow(ctx, `
select count(*) from quota_snapshots
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4
  and stale_after <= $5`,
			scope.TenantID, int32(scope.Provider), scope.ResourceType, scope.ResourceID, now,
		).Scan(&stale)
		if err != nil {
			return false, err
		}
		if stale > 0 {
			return true, nil
		}
		var total int
		err = s.db.Pool.QueryRow(ctx, `
select count(*) from quota_snapshots
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4`,
			scope.TenantID, int32(scope.Provider), scope.ResourceType, scope.ResourceID,
		).Scan(&total)
		return total == 0, err
	}
	for _, name := range quotaNames {
		var staleAfter time.Time
		err := s.db.Pool.QueryRow(ctx, `
select stale_after from quota_snapshots
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4 and quota_name = $5`,
			scope.TenantID, int32(scope.Provider), scope.ResourceType, scope.ResourceID, name,
		).Scan(&staleAfter)
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if !staleAfter.After(now) {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) ListQuotaViews(ctx context.Context, tenantID string, provider deploymentpb.Provider, now time.Time) ([]*api.QuotaView, error) {
	args := []any{
		tenantID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
		now,
		int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
	}
	whereProvider := ""
	if provider != deploymentpb.Provider_PROVIDER_UNSPECIFIED {
		args = append(args, int32(provider))
		whereProvider = fmt.Sprintf(" and s.provider = $%d", len(args))
	}
	rows, err := s.db.Pool.Query(ctx, `
select s.provider, s.resource_type, s.resource_id, s.service, s.quota_name, s.units,
       s.provider_used::float8, s.quota_limit::float8, s.provider_available::float8,
       coalesce(sum(r.amount) filter (
         where r.status = $2 and (r.expires_at is null or r.expires_at > $3)
            or r.status = $4 and r.updated_at > s.observed_at
       ), 0)::float8 as reserved,
       s.observed_at, s.stale_after
from quota_snapshots s
left join quota_reservations r
  on r.tenant_id = s.tenant_id and r.provider = s.provider
 and r.resource_type = s.resource_type and r.resource_id = s.resource_id
 and r.quota_name = s.quota_name
where s.tenant_id = $1`+whereProvider+`
group by s.tenant_id, s.provider, s.resource_type, s.resource_id, s.service, s.quota_name,
         s.units, s.provider_used, s.quota_limit, s.provider_available, s.observed_at, s.stale_after
order by s.provider, s.service, s.quota_name`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*api.QuotaView, 0)
	for rows.Next() {
		var (
			p                                 int32
			resourceType, resourceID, service string
			name, units                       string
			used, limit, providerAvailable    float64
			reserved                          float64
			observedAt, staleAfter            time.Time
		)
		if err := rows.Scan(&p, &resourceType, &resourceID, &service, &name, &units, &used, &limit, &providerAvailable, &reserved, &observedAt, &staleAfter); err != nil {
			return nil, err
		}
		available := providerAvailable - reserved
		if available < 0 {
			available = 0
		}
		out = append(out, &api.QuotaView{
			Info: &deploymentpb.Quota_Info{
				Provider: deploymentpb.Provider(p),
				Name:     name,
				Units:    units,
			},
			ResourceType:      resourceType,
			ResourceId:        resourceID,
			Service:           service,
			ProviderUsed:      used,
			Limit:             limit,
			ProviderAvailable: providerAvailable,
			Reserved:          reserved,
			AvailableForRuns:  available,
			ObservedAt:        timestamppb.New(observedAt),
			StaleAfter:        timestamppb.New(staleAfter),
			Stale:             !staleAfter.After(now),
		})
	}
	return out, rows.Err()
}

func (s *Store) ListRunReservations(ctx context.Context, tenantID, runID string) ([]Reservation, error) {
	rows, err := s.db.Pool.Query(ctx, `
select id, tenant_id, run_id, node_id, provider, resource_type, resource_id, service,
       quota_name, units, amount::float8, status, workflow_id, expires_at, created_at, updated_at
from quota_reservations
where tenant_id = $1 and run_id = $2
order by node_id, quota_name`,
		tenantID, runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReservations(rows)
}

// ReserveInput is Store.Reserve's argument: Requests is the remodeled
// replacement for the deleted []*workflowpb.QuotaRequestRef — one
// QuotaAmount per resource kind (cpu/ram/disk), already aggregated across
// every machine_groups entry and expressed in the units Scope's provider
// reports quota in (see reserve.go's buildQuotaAmounts, the Manager-level
// caller that builds this list from a dslpb.CompiledPlan).
type ReserveInput struct {
	TenantID       string
	RunID          string
	WorkflowID     string
	Scope          Scope
	Requests       []QuotaAmount
	ReservationTTL time.Duration
	Now            time.Time
}

// Reserve writes one quota_reservations row per ReserveInput.Requests entry
// (keyed by tenant/run/quota_name — NodeID is always "" since a
// machine-groups-derived demand is a run-level aggregate, not a per-machine
// one), failing the whole reservation (no rows written — the surrounding
// transaction rolls back) if any requested kind's amount would exceed that
// kind's live AvailableForRuns (provider_available minus every OTHER run's
// still-active reservation for the same quota_name). Recovered verbatim from
// the pre-DSL-pivot Store.Reserve (git show efd9bffc^) with the input
// re-plumbed from []*workflowpb.QuotaRequestRef (deleted, per-node) to
// []QuotaAmount (per-kind aggregate) — see reserve.go's package doc for the
// full remodel rationale.
func (s *Store) Reserve(ctx context.Context, input ReserveInput) ([]Reservation, error) {
	if input.ReservationTTL <= 0 {
		input.ReservationTTL = 30 * time.Minute
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	tx, err := s.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)
	if err := lockScope(ctx, tx, input.Scope); err != nil {
		return nil, err
	}

	// Expire any reservation in this scope whose TTL has already lapsed
	// before checking availability, so a stale RESERVED row from an
	// abandoned run does not falsely hold capacity hostage.
	if _, err := tx.Exec(ctx, `
update quota_reservations set status = $1, updated_at = $2
where tenant_id = $3 and provider = $4 and resource_type = $5 and resource_id = $6
  and status = $7 and expires_at is not null and expires_at <= $2`,
		int32(deploymentpb.Quota_RESERVATION_STATUS_EXPIRED),
		now,
		input.TenantID,
		int32(input.Scope.Provider),
		input.Scope.ResourceType,
		input.Scope.ResourceID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
	); err != nil {
		return nil, err
	}

	for _, req := range input.Requests {
		var providerAvailable float64
		var observedAt, staleAfter time.Time
		err := tx.QueryRow(ctx, `
select provider_available::float8, observed_at, stale_after from quota_snapshots
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4 and quota_name = $5`,
			input.TenantID,
			int32(input.Scope.Provider),
			input.Scope.ResourceType,
			input.Scope.ResourceID,
			req.QuotaName,
		).Scan(&providerAvailable, &observedAt, &staleAfter)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSnapshotMissing
		}
		if err != nil {
			return nil, err
		}
		if !staleAfter.After(now) {
			return nil, ErrSnapshotStale
		}

		var otherReserved float64
		err = tx.QueryRow(ctx, `
select coalesce(sum(amount), 0)::float8
from quota_reservations
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4
  and quota_name = $5 and run_id <> $6
  and (
    status = $7 and (expires_at is null or expires_at > $8)
    or status = $9 and updated_at > $10
  )`,
			input.TenantID,
			int32(input.Scope.Provider),
			input.Scope.ResourceType,
			input.Scope.ResourceID,
			req.QuotaName,
			input.RunID,
			int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
			now,
			int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
			observedAt,
		).Scan(&otherReserved)
		if err != nil {
			return nil, err
		}
		if err := checkAvailable(req.QuotaName, providerAvailable, otherReserved, req.Amount); err != nil {
			return nil, err
		}
	}

	expiresAt := now.Add(input.ReservationTTL)
	for _, req := range input.Requests {
		if req.Amount == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
insert into quota_reservations
  (id, tenant_id, run_id, node_id, provider, resource_type, resource_id, service,
   quota_name, units, amount, status, workflow_id, expires_at, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)
on conflict (tenant_id, run_id, node_id, provider, resource_type, resource_id, quota_name)
do update set
  service = excluded.service,
  units = excluded.units,
  amount = excluded.amount,
  status = excluded.status,
  workflow_id = excluded.workflow_id,
  expires_at = excluded.expires_at,
  updated_at = excluded.updated_at`,
			uuid.NewString(),
			input.TenantID,
			input.RunID,
			"",
			int32(input.Scope.Provider),
			input.Scope.ResourceType,
			input.Scope.ResourceID,
			input.Scope.Service,
			req.QuotaName,
			req.Units,
			req.Amount,
			int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
			input.WorkflowID,
			expiresAt,
			now,
		); err != nil {
			return nil, err
		}
	}

	rows, err := tx.Query(ctx, `
select id, tenant_id, run_id, node_id, provider, resource_type, resource_id, service,
       quota_name, units, amount::float8, status, workflow_id, expires_at, created_at, updated_at
from quota_reservations
where tenant_id = $1 and run_id = $2
  and status in ($3, $4)
order by node_id, quota_name`,
		input.TenantID,
		input.RunID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
		int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
	)
	if err != nil {
		return nil, err
	}
	reservations, err := scanReservations(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return reservations, nil
}

// CommitRun promotes every RESERVED reservation for (tenantID, runID) to
// ALLOCATED — called once a recipe run's ExecuteCompiledPlanWorkflow child
// succeeds (see runrecipe.go's commitQuotas). Recovered verbatim from the
// pre-DSL-pivot Store (git show efd9bffc^) — run-id keyed, so it needed no
// remodel.
func (s *Store) CommitRun(ctx context.Context, tenantID, runID string, now time.Time) ([]Reservation, error) {
	if now.IsZero() {
		now = time.Now()
	}
	tx, err := s.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `
update quota_reservations
set status = $1, updated_at = $2
where tenant_id = $3 and run_id = $4 and status = $5`,
		int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
		now,
		tenantID,
		runID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
	); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
select id, tenant_id, run_id, node_id, provider, resource_type, resource_id, service,
       quota_name, units, amount::float8, status, workflow_id, expires_at, created_at, updated_at
from quota_reservations
where tenant_id = $1 and run_id = $2 and status = $3
order by node_id, quota_name`,
		tenantID,
		runID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
	)
	if err != nil {
		return nil, err
	}
	reservations, err := scanReservations(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return reservations, nil
}

// ReleaseRun marks every RESERVED or ALLOCATED reservation for (tenantID,
// runID) RELEASED — called unconditionally from RunRecipeWorkflow's teardown
// defer (success, failure, or cancellation: see runrecipe.go's releaseQuotas)
// so a run's demand always frees the capacity it held, whether or not it
// ever reached CommitRun. Recovered verbatim from the pre-DSL-pivot Store
// (git show efd9bffc^) plus the RESERVED-or-ALLOCATED status widening a
// later commit (d2db22cc, "Release quotas after teardown") already applied
// before this code was deleted.
func (s *Store) ReleaseRun(ctx context.Context, tenantID, runID string, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now()
	}
	tag, err := s.db.Pool.Exec(ctx, `
update quota_reservations
set status = $1, updated_at = $2
where tenant_id = $3 and run_id = $4 and status in ($5, $6)`,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RELEASED),
		now,
		tenantID,
		runID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
		int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
	)
	return tag.RowsAffected(), err
}

// checkAvailable is Store.Reserve's over-limit decision, extracted as a pure
// function (no DB access) specifically so it is unit-testable without a
// database harness (this repo has none for postgres.DB — see reserve_test.go)
// — mirrors the recovered pre-DSL-pivot Store.Reserve's identical inline
// check (git show efd9bffc^). Returns ErrInsufficient (wrapped with a
// human-readable amount) once (providerAvailable - otherReserved) can no
// longer cover amount, nil otherwise.
func checkAvailable(quotaName string, providerAvailable, otherReserved float64, amount uint64) error {
	if providerAvailable-otherReserved < float64(amount) {
		return fmt.Errorf("%w: quota %s has %.0f available, needs %d", ErrInsufficient, quotaName, math.Max(providerAvailable-otherReserved, 0), amount)
	}
	return nil
}

// lockScope takes a transaction-scoped advisory lock keyed by (tenant,
// provider:resource_type:resource_id) so concurrent Reserve calls against the
// same quota scope serialize instead of racing the
// available-capacity-vs-other-reservations check above. Recovered verbatim
// from the pre-DSL-pivot Store (git show efd9bffc^).
func lockScope(ctx context.Context, tx pgx.Tx, scope Scope) error {
	_, err := tx.Exec(ctx,
		`select pg_advisory_xact_lock(hashtext($1), hashtext($2))`,
		scope.TenantID,
		fmt.Sprintf("%d:%s:%s", scope.Provider, scope.ResourceType, scope.ResourceID),
	)
	return err
}

func scanReservations(rows pgx.Rows) ([]Reservation, error) {
	defer rows.Close()
	out := make([]Reservation, 0)
	for rows.Next() {
		var (
			r         Reservation
			provider  int32
			amount    float64
			status    int32
			expiresAt *time.Time
		)
		if err := rows.Scan(
			&r.ID,
			&r.TenantID,
			&r.RunID,
			&r.NodeID,
			&provider,
			&r.ResourceType,
			&r.ResourceID,
			&r.Service,
			&r.QuotaName,
			&r.Units,
			&amount,
			&status,
			&r.WorkflowID,
			&expiresAt,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.Provider = deploymentpb.Provider(provider)
		r.Amount = uint64(math.Ceil(amount))
		r.Status = deploymentpb.Quota_ReservationStatus(status)
		r.ExpiresAt = expiresAt
		out = append(out, r)
	}
	return out, rows.Err()
}

func ReservationsToViews(reservations []Reservation) []*api.QuotaReservationView {
	out := make([]*api.QuotaReservationView, 0, len(reservations))
	for _, r := range reservations {
		view := &api.QuotaReservationView{
			Id:           r.ID,
			RunId:        r.RunID,
			NodeId:       r.NodeID,
			Info:         &deploymentpb.Quota_Info{Provider: r.Provider, Name: r.QuotaName, Units: r.Units},
			ResourceType: r.ResourceType,
			ResourceId:   r.ResourceID,
			Service:      r.Service,
			Amount:       r.Amount,
			Status:       r.Status,
			CreatedAt:    timestamppb.New(r.CreatedAt),
			UpdatedAt:    timestamppb.New(r.UpdatedAt),
		}
		if r.ExpiresAt != nil {
			view.ExpiresAt = timestamppb.New(*r.ExpiresAt)
		}
		out = append(out, view)
	}
	return out
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}
