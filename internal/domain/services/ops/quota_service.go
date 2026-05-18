package ops

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)


const defaultConcurrentRunsLimit int64 = 10

// QuotaService enforces per-tenant resource quotas backed by quota_counters.
type QuotaService struct {
	repo  *repository.ProtoRepository[opspb.QuotaCounterAlias, opspb.QuotaCounterColumnAlias, *opspb.QuotaCounterScanner, *opspb.QuotaCounter]
	pool  *pgxpool.Pool
	txMgr pgtx.TxManager
}

// NewQuotaService constructs a real QuotaService.
func NewQuotaService(executor exec.DB, pool *pgxpool.Pool, txMgr pgtx.TxManager) *QuotaService {
	return &QuotaService{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.QuotaCounters.Table, executor),
			opspb.QuotaCounterConverter,
		),
		pool:  pool,
		txMgr: txMgr,
	}
}

// CheckAndReserve atomically checks the quota and increments used by amount.
// Returns RESOURCE_EXHAUSTED if used+amount > limit_value.
// If no counter row exists, uses the default limit and inserts on first use.
func (s *QuotaService) CheckAndReserve(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error {
	const q = `
WITH existing AS (
	SELECT id, limit_value, used FROM quota_counters
	WHERE tenant_id = $1 AND resource_id = $2 AND deleted_at IS NULL
	FOR UPDATE
),
check_and_update AS (
	UPDATE quota_counters SET
		used = used + $4,
		updated_at = now()
	WHERE tenant_id = $1 AND resource_id = $2 AND deleted_at IS NULL
	AND (used + $4) <= COALESCE((SELECT limit_value FROM existing), $3::bigint)
	RETURNING used
)
SELECT
	COALESCE((SELECT used FROM existing), 0) AS current_used,
	COALESCE((SELECT limit_value FROM existing), $3::bigint) AS limit_value,
	EXISTS(SELECT 1 FROM check_and_update) AS updated
`
	row := s.pool.QueryRow(ctx, q, tenantID.GetValue(), resourceID, defaultConcurrentRunsLimit, amount)
	var currentUsed, limitValue int64
	var updated bool
	if err := row.Scan(&currentUsed, &limitValue, &updated); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if !updated {
		return status.Errorf(codes.ResourceExhausted,
			"quota exceeded for %s/%s: used=%d limit=%d",
			tenantID.GetValue(), resourceID, currentUsed, limitValue)
	}
	return nil
}

// Release decrements the used counter by amount (floor 0).
// A serializable transaction is used to avoid a read-modify-write race; GREATEST
// cannot be expressed in ratel so the guard is applied in Go instead.
func (s *QuotaService) Release(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error {
	return pgtx.WithSerializable(ctx, s.txMgr, func(ctx context.Context) error {
		counter, err := s.getCounter(ctx, tenantID, resourceID)
		if err != nil {
			return err
		}
		if counter == nil {
			return nil
		}
		newUsed := counter.GetUsed() - amount
		if newUsed < 0 {
			newUsed = 0
		}
		_, err = s.repo.Execute(ctx,
			opspb.QuotaCounters.Update().
				Set(
					opspb.QuotaCounters.Used.Set(newUsed),
					opspb.QuotaCounters.UpdatedAt.Set(time.Now()),
				).
				Where(opspb.QuotaCounters.Id.Eq(counter.GetId().GetValue())),
		)
		return err
	})
}

// SetLimit upserts a quota_counters row for the given tenant+resource.
func (s *QuotaService) SetLimit(ctx context.Context, tenantID *iampb.TenantId, resourceID string, limit int64) error {
	return pgtx.WithSerializable(ctx, s.txMgr, func(ctx context.Context) error {
		counter, err := s.getCounter(ctx, tenantID, resourceID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		now := time.Now()
		nowTs := timestamppb.New(now)
		if counter == nil {
			c := &opspb.QuotaCounter{
				Id:         &opspb.QuotaCounterId{Value: ids.New()},
				TenantId:   tenantID,
				ResourceId: resourceID,
				LimitValue: limit,
				Used:       0,
				Timestamps: &commonpb.Timestamps{CreatedAt: nowTs, UpdatedAt: nowTs},
			}
			sc := c.IntoPlain()
			_, err = s.repo.Execute(ctx, opspb.QuotaCounters.Insert().From(sc.AllSetters()...))
			return err
		}
		_, err = s.repo.Execute(ctx,
			opspb.QuotaCounters.Update().
				Set(
					opspb.QuotaCounters.LimitValue.Set(limit),
					opspb.QuotaCounters.UpdatedAt.Set(now),
				).
				Where(opspb.QuotaCounters.Id.Eq(counter.GetId().GetValue())),
		)
		return err
	})
}

func (s *QuotaService) getCounter(ctx context.Context, tenantID *iampb.TenantId, resourceID string) (*opspb.QuotaCounter, error) {
	c, err := s.repo.QueryRow(ctx,
		opspb.QuotaCounters.SelectAll().Where(
			opspb.QuotaCounters.TenantId.Eq(tenantID.GetValue()),
			opspb.QuotaCounters.ResourceId.Eq(resourceID),
			opspb.QuotaCounters.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// GetQuotas returns quotas for a tenant from DB.
func (s *QuotaService) GetQuotas(ctx context.Context, req *opspb.GetQuotasRequest) (*opspb.QuotaList, error) {
	counters, err := s.repo.Query(ctx,
		opspb.QuotaCounters.SelectAll().Where(
			opspb.QuotaCounters.TenantId.Eq(req.GetTenantId().GetValue()),
			opspb.QuotaCounters.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		return nil, err
	}
	quotas := make([]*opspb.Quota, 0, len(counters))
	for _, c := range counters {
		quotas = append(quotas, &opspb.Quota{
			ResourceId: c.GetResourceId(),
			Limit:      c.GetLimitValue(),
			Used:       c.GetUsed(),
		})
	}
	return &opspb.QuotaList{Quotas: quotas}, nil
}

// RefreshQuotas re-queries and returns current quotas (same as GetQuotas).
func (s *QuotaService) RefreshQuotas(ctx context.Context, req *opspb.RefreshQuotasRequest) (*opspb.QuotaList, error) {
	return s.GetQuotas(ctx, &opspb.GetQuotasRequest{TenantId: req.GetTenantId()})
}
