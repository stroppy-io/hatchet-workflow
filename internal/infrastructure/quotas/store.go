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

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

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

	aggregate := aggregateRequests(input.QuotaRequests)
	for quotaName, amount := range aggregate {
		var providerAvailable float64
		var observedAt, staleAfter time.Time
		err := tx.QueryRow(ctx, `
select provider_available::float8, observed_at, stale_after from quota_snapshots
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4 and quota_name = $5`,
			input.TenantID,
			int32(input.Scope.Provider),
			input.Scope.ResourceType,
			input.Scope.ResourceID,
			quotaName,
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
			quotaName,
			input.RunID,
			int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
			now,
			int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
			observedAt,
		).Scan(&otherReserved)
		if err != nil {
			return nil, err
		}
		if providerAvailable-otherReserved < float64(amount) {
			return nil, derrors.FailedPrecondition("QUOTA_INSUFFICIENT", fmt.Sprintf("quota %s has %.0f available, needs %d", quotaName, math.Max(providerAvailable-otherReserved, 0), amount)).Wrap(ErrInsufficient)
		}
	}

	expiresAt := now.Add(input.ReservationTTL)
	for _, ref := range input.QuotaRequests {
		req := ref.GetRequest()
		info := req.GetInfo()
		if info == nil || req.GetRequest() == 0 {
			continue
		}
		service := input.Scope.Service
		if service == "" {
			service = serviceFromQuota(info.GetName())
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
			ref.GetNodeId(),
			int32(input.Scope.Provider),
			input.Scope.ResourceType,
			input.Scope.ResourceID,
			service,
			info.GetName(),
			unitsOrInfer(info.GetUnits(), info.GetName()),
			req.GetRequest(),
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

func (s *Store) ReleaseRun(ctx context.Context, tenantID, runID string, now time.Time) (uint32, error) {
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
	return uint32(tag.RowsAffected()), err
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

func ReservationsToAllocationRefs(reservations []Reservation) []*workflowpb.QuotaAllocationRef {
	out := make([]*workflowpb.QuotaAllocationRef, 0, len(reservations))
	for _, r := range reservations {
		out = append(out, &workflowpb.QuotaAllocationRef{
			NodeId: r.NodeID,
			Allocation: &deploymentpb.Quota_Allocation{
				Info: &deploymentpb.Quota_Info{
					Provider: r.Provider,
					Name:     r.QuotaName,
					Units:    r.Units,
				},
				Used: r.Amount,
			},
		})
	}
	return out
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

func lockScope(ctx context.Context, tx pgx.Tx, scope Scope) error {
	_, err := tx.Exec(ctx,
		`select pg_advisory_xact_lock(hashtext($1), hashtext($2))`,
		scope.TenantID,
		fmt.Sprintf("%d:%s:%s", scope.Provider, scope.ResourceType, scope.ResourceID),
	)
	return err
}

func aggregateRequests(refs []*workflowpb.QuotaRequestRef) map[string]uint64 {
	out := make(map[string]uint64)
	for _, ref := range refs {
		req := ref.GetRequest()
		info := req.GetInfo()
		if info == nil || req.GetRequest() == 0 {
			continue
		}
		out[info.GetName()] += req.GetRequest()
	}
	return out
}
