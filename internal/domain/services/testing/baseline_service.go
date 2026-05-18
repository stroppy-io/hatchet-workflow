package testing

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// BaselineService owns the named TestRun pointers a tenant uses as comparison
// targets in regression dashboards. UNIQUE(tenant_id, name) — Set is an upsert.
type BaselineService struct {
	*tracing.Entity

	repo   *repository.ProtoRepository[testingpb.BaselineAlias, testingpb.BaselineColumnAlias, *testingpb.BaselineScanner, *testingpb.Baseline]
	txMgr  pgtx.TxManager
	events eventing.Bus
}

// NewBaselineService wires the service against the baselines table.
func NewBaselineService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *BaselineService {
	return &BaselineService{
		Entity: tracing.NewEntity("testing.BaselineService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.Baselines.Table, executor),
			testingpb.BaselineConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}

// SetBaseline upserts the (tenant_id, name) row to point at test_run_id.
func (s *BaselineService) SetBaseline(ctx context.Context, tenantID *iampb.TenantId, name string, testRunID *testingpb.TestRunId, createdBy *iampb.UserId) (*testingpb.Baseline, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "SetBaseline",
		func(ctx context.Context, _ trace.Span) (*testingpb.Baseline, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.Baseline, error) {
					now := timestamppb.Now()
					existing, err := s.repo.QueryRow(ctx,
						testingpb.Baselines.SelectAll().Where(
							testingpb.Baselines.TenantId.Eq(tenantID.GetValue()),
							testingpb.Baselines.Name.Eq(name),
							testingpb.Baselines.DeletedAt.IsNull(),
						),
					)
					if err == nil {
						existing.TestRunId = testRunID
						existing.GetTimestamps().UpdatedAt = now
						scanner := existing.IntoPlain()
						_, err := s.repo.Execute(ctx,
							testingpb.Baselines.Update().
								Set(
									scanner.GetSetter(testingpb.BaselineColumnTestRunId)(),
									scanner.GetSetter(testingpb.BaselineColumnUpdatedAt)(),
								).
								Where(testingpb.Baselines.Id.Eq(existing.GetId().GetValue())),
						)
						if err != nil {
							return nil, err
						}
						return existing, nil
					}
					if !errors.Is(err, pgx.ErrNoRows) {
						return nil, err
					}
					row := &testingpb.Baseline{
						Id:        &testingpb.BaselineId{Value: ids.New()},
						TenantId:  tenantID,
						Name:      name,
						TestRunId: testRunID,
						CreatedBy: createdBy,
						Timestamps: &commonpb.Timestamps{
							CreatedAt: now,
							UpdatedAt: now,
						},
					}
					scanner := row.IntoPlain()
					if _, err := s.repo.Execute(ctx,
						testingpb.Baselines.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return row, nil
				})
		})
}

// GetBaseline fetches by (tenant_id, name).
func (s *BaselineService) GetBaseline(ctx context.Context, tenantID *iampb.TenantId, name string) (*testingpb.Baseline, error) {
	b, err := s.repo.QueryRow(ctx,
		testingpb.Baselines.SelectAll().Where(
			testingpb.Baselines.TenantId.Eq(tenantID.GetValue()),
			testingpb.Baselines.Name.Eq(name),
			testingpb.Baselines.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("baseline", name))
		}
		return nil, err
	}
	return b, nil
}

// ListBaselines returns all baselines for a tenant.
func (s *BaselineService) ListBaselines(ctx context.Context, tenantID *iampb.TenantId) ([]*testingpb.Baseline, error) {
	return s.repo.Query(ctx,
		testingpb.Baselines.SelectAll().Where(
			testingpb.Baselines.TenantId.Eq(tenantID.GetValue()),
			testingpb.Baselines.DeletedAt.IsNull(),
		),
	)
}

// DeleteBaseline soft-deletes a row, returning the pre-delete record.
func (s *BaselineService) DeleteBaseline(ctx context.Context, id *testingpb.BaselineId) (*testingpb.Baseline, error) {
	b, err := s.repo.QueryRow(ctx,
		testingpb.Baselines.SelectAll().Where(
			testingpb.Baselines.Id.Eq(id.GetValue()),
			testingpb.Baselines.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("baseline", id.GetValue()))
		}
		return nil, err
	}
	nowT := timestamppb.Now().AsTime()
	_, err = s.repo.Execute(ctx,
		testingpb.Baselines.Update().
			Set(testingpb.Baselines.DeletedAt.Set(&nowT)).
			Where(testingpb.Baselines.Id.Eq(id.GetValue())),
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}
