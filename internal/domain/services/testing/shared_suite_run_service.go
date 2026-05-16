package testing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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

// safeSelectSharedSuiteRunCols lists columns safe to scan for shared_suite_runs.
var safeSelectSharedSuiteRunCols = []testingpb.SharedSuiteRunColumnAlias{
	testingpb.SharedSuiteRunColumnId,
	testingpb.SharedSuiteRunColumnTenantId,
	testingpb.SharedSuiteRunColumnCreatedAt,
	testingpb.SharedSuiteRunColumnUpdatedAt,
	testingpb.SharedSuiteRunColumnDeletedAt,
	testingpb.SharedSuiteRunColumnTestSuiteRunId,
	testingpb.SharedSuiteRunColumnCreatedBy,
	testingpb.SharedSuiteRunColumnToken,
	testingpb.SharedSuiteRunColumnSnapshot,
	testingpb.SharedSuiteRunColumnMetrics,
	testingpb.SharedSuiteRunColumnExpiresAt,
	testingpb.SharedSuiteRunColumnViewCount,
}

// SharedSuiteRunService manages shared suite-run links.
type SharedSuiteRunService struct {
	*tracing.Entity

	repo  *repository.ProtoRepository[testingpb.SharedSuiteRunAlias, testingpb.SharedSuiteRunColumnAlias, *testingpb.SharedSuiteRunScanner, *testingpb.SharedSuiteRun]
	txMgr pgtx.TxManager
	//nolint:unused
	events eventing.Bus
}

// NewSharedSuiteRunService constructs a SharedSuiteRunService.
func NewSharedSuiteRunService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *SharedSuiteRunService {
	return &SharedSuiteRunService{
		Entity: tracing.NewEntity("testing.SharedSuiteRunService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.SharedSuiteRuns.Table, executor),
			testingpb.SharedSuiteRunConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}

// Create mints a new shared link for testSuiteRunID, valid for ttl (0 = no expiry).
func (s *SharedSuiteRunService) Create(
	ctx context.Context,
	tenantID *iampb.TenantId,
	callerID *iampb.UserId,
	testSuiteRunID *testingpb.TestSuiteRunId,
	ttl time.Duration,
) (*testingpb.SharedSuiteRun, error) {
	return pgtx.WithSerializableRet(ctx, s.txMgr,
		func(ctx context.Context) (*testingpb.SharedSuiteRun, error) {
			token, err := newToken()
			if err != nil {
				return nil, err
			}

			now := timestamppb.Now()
			shared := &testingpb.SharedSuiteRun{
				Id:             &testingpb.SharedSuiteRunId{Value: ids.New()},
				TenantId:       tenantID,
				Timestamps:     &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
				TestSuiteRunId: testSuiteRunID,
				CreatedBy:      callerID,
				Token:          token,
			}
			if ttl > 0 {
				expiresAt := time.Now().Add(ttl)
				shared.ExpiresAt = timestamppb.New(expiresAt)
			}

			scanner := shared.IntoPlain()
			if len(scanner.Snapshot) == 0 {
				scanner.Snapshot = []byte("{}")
			}
			if len(scanner.Metrics) == 0 {
				scanner.Metrics = []byte("{}")
			}

			if _, err := s.repo.Execute(ctx,
				testingpb.SharedSuiteRuns.Insert().From(scanner.AllSetters()...),
			); err != nil {
				return nil, err
			}
			return shared, nil
		})
}

// Revoke soft-deletes a SharedSuiteRun by setting deleted_at.
func (s *SharedSuiteRunService) Revoke(
	ctx context.Context,
	id *testingpb.SharedSuiteRunId,
) (*testingpb.SharedSuiteRun, error) {
	existing, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	n, err := s.repo.Execute(ctx,
		testingpb.SharedSuiteRuns.Update().
			Set(testingpb.SharedSuiteRuns.DeletedAt.Set(&now)).
			Where(
				testingpb.SharedSuiteRuns.Id.Eq(id.GetValue()),
				testingpb.SharedSuiteRuns.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_suite_run", id.GetValue()))
	}
	return existing, nil
}

// GetByToken looks up a non-deleted, non-expired SharedSuiteRun by token.
func (s *SharedSuiteRunService) GetByToken(
	ctx context.Context,
	token string,
) (*testingpb.SharedSuiteRun, error) {
	p, err := s.repo.QueryRow(ctx,
		testingpb.SharedSuiteRuns.Select(safeSelectSharedSuiteRunCols...).Where(
			testingpb.SharedSuiteRuns.Token.Eq(token),
			testingpb.SharedSuiteRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_suite_run", token))
		}
		return nil, err
	}
	if ea := p.GetExpiresAt(); ea != nil {
		if ea.AsTime().Before(time.Now()) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_suite_run", token))
		}
	}
	return p, nil
}

// ListByTenant returns all non-deleted SharedSuiteRuns for tenantID.
func (s *SharedSuiteRunService) ListByTenant(
	ctx context.Context,
	tenantID *iampb.TenantId,
) ([]*testingpb.SharedSuiteRun, error) {
	return s.repo.Query(ctx,
		testingpb.SharedSuiteRuns.Select(safeSelectSharedSuiteRunCols...).Where(
			testingpb.SharedSuiteRuns.TenantId.Eq(tenantID.GetValue()),
			testingpb.SharedSuiteRuns.DeletedAt.IsNull(),
		),
	)
}

func (s *SharedSuiteRunService) getByID(
	ctx context.Context,
	id *testingpb.SharedSuiteRunId,
) (*testingpb.SharedSuiteRun, error) {
	p, err := s.repo.QueryRow(ctx,
		testingpb.SharedSuiteRuns.Select(safeSelectSharedSuiteRunCols...).Where(
			testingpb.SharedSuiteRuns.Id.Eq(id.GetValue()),
			testingpb.SharedSuiteRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_suite_run", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}
