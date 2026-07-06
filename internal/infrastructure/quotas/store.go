package quotas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
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
