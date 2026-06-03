package networks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

type Store struct {
	db *postgres.DB
}

func NewStore(db *postgres.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Reserve(ctx context.Context, input ReserveInput) (*Reservation, error) {
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
update network_reservations set status = $1, updated_at = $2
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

	existing, err := selectRunReservation(ctx, tx, input.TenantID, input.RunID)
	if err == nil && isActive(existing, now) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	activeCIDRs, err := activeReservationCIDRs(ctx, tx, input, now)
	if err != nil {
		return nil, err
	}
	used := append([]string{}, input.ProviderCIDRs...)
	used = append(used, activeCIDRs...)
	cidr, err := SelectRunCIDR(input.RunID, input.BaseCIDR, used)
	if err != nil {
		return nil, derrors.FailedPrecondition("NETWORK_CIDR_EXHAUSTED", err.Error()).Wrap(err)
	}

	expiresAt := now.Add(input.ReservationTTL)
	if _, err := tx.Exec(ctx, `
insert into network_reservations
  (id, tenant_id, run_id, provider, resource_type, resource_id, cidr, status, workflow_id, expires_at, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)
on conflict (tenant_id, run_id, provider, resource_type, resource_id)
do update set
  cidr = excluded.cidr,
  status = excluded.status,
  workflow_id = excluded.workflow_id,
  expires_at = excluded.expires_at,
  updated_at = excluded.updated_at`,
		uuid.NewString(),
		input.TenantID,
		input.RunID,
		int32(input.Scope.Provider),
		input.Scope.ResourceType,
		input.Scope.ResourceID,
		cidr,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
		input.WorkflowID,
		expiresAt,
		now,
	); err != nil {
		return nil, err
	}

	reservation, err := selectRunReservation(ctx, tx, input.TenantID, input.RunID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return reservation, nil
}

func (s *Store) CommitRun(ctx context.Context, tenantID, runID string, now time.Time) (*Reservation, error) {
	if now.IsZero() {
		now = time.Now()
	}
	tx, err := s.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `
update network_reservations
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
	reservation, err := selectRunReservation(ctx, tx, tenantID, runID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return reservation, nil
}

func (s *Store) ReleaseRun(ctx context.Context, tenantID, runID string, now time.Time) (uint32, error) {
	if now.IsZero() {
		now = time.Now()
	}
	tag, err := s.db.Pool.Exec(ctx, `
update network_reservations
set status = $1, updated_at = $2
where tenant_id = $3 and run_id = $4 and status = $5`,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RELEASED),
		now,
		tenantID,
		runID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
	)
	return uint32(tag.RowsAffected()), err
}

func activeReservationCIDRs(ctx context.Context, tx pgx.Tx, input ReserveInput, now time.Time) ([]string, error) {
	rows, err := tx.Query(ctx, `
select cidr
from network_reservations
where tenant_id = $1 and provider = $2 and resource_type = $3 and resource_id = $4
  and run_id <> $5
  and (
    status = $6 and (expires_at is null or expires_at > $7)
    or status = $8
  )`,
		input.TenantID,
		int32(input.Scope.Provider),
		input.Scope.ResourceType,
		input.Scope.ResourceID,
		input.RunID,
		int32(deploymentpb.Quota_RESERVATION_STATUS_RESERVED),
		now,
		int32(deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			return nil, err
		}
		out = append(out, cidr)
	}
	return out, rows.Err()
}

func selectRunReservation(ctx context.Context, tx pgx.Tx, tenantID, runID string) (*Reservation, error) {
	row := tx.QueryRow(ctx, `
select id, tenant_id, run_id, provider, resource_type, resource_id, cidr, status,
       workflow_id, expires_at, created_at, updated_at
from network_reservations
where tenant_id = $1 and run_id = $2
order by updated_at desc
limit 1`,
		tenantID,
		runID,
	)
	return scanReservation(row)
}

func scanReservation(row pgx.Row) (*Reservation, error) {
	var (
		r         Reservation
		provider  int32
		status    int32
		expiresAt *time.Time
	)
	if err := row.Scan(
		&r.ID,
		&r.TenantID,
		&r.RunID,
		&provider,
		&r.ResourceType,
		&r.ResourceID,
		&r.CIDR,
		&status,
		&r.WorkflowID,
		&expiresAt,
		&r.CreatedAt,
		&r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	r.Provider = deploymentpb.Provider(provider)
	r.Status = deploymentpb.Quota_ReservationStatus(status)
	r.ExpiresAt = expiresAt
	return &r, nil
}

func isActive(r *Reservation, now time.Time) bool {
	if r == nil {
		return false
	}
	if r.Status == deploymentpb.Quota_RESERVATION_STATUS_ALLOCATED {
		return true
	}
	return r.Status == deploymentpb.Quota_RESERVATION_STATUS_RESERVED && (r.ExpiresAt == nil || r.ExpiresAt.After(now))
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
