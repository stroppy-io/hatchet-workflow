package testing

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

// safeSelectSharedRunCols lists columns safe to scan for shared_test_runs.
var safeSelectSharedRunCols = []testingpb.SharedTestRunColumnAlias{
	testingpb.SharedTestRunColumnId,
	testingpb.SharedTestRunColumnTenantId,
	testingpb.SharedTestRunColumnCreatedAt,
	testingpb.SharedTestRunColumnUpdatedAt,
	testingpb.SharedTestRunColumnDeletedAt,
	testingpb.SharedTestRunColumnTestRunId,
	testingpb.SharedTestRunColumnCreatedBy,
	testingpb.SharedTestRunColumnToken,
	testingpb.SharedTestRunColumnSnapshot,
	testingpb.SharedTestRunColumnMetrics,
	testingpb.SharedTestRunColumnExpiresAt,
	testingpb.SharedTestRunColumnViewCount,
}

// SharedTestRunService manages shared test-run links.
type SharedTestRunService struct {
	*tracing.Entity

	repo  *repository.ProtoRepository[testingpb.SharedTestRunAlias, testingpb.SharedTestRunColumnAlias, *testingpb.SharedTestRunScanner, *testingpb.SharedTestRun]
	txMgr pgtx.TxManager
	//nolint:unused
	events eventing.Bus
}

// NewSharedTestRunService constructs a SharedTestRunService.
func NewSharedTestRunService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *SharedTestRunService {
	return &SharedTestRunService{
		Entity: tracing.NewEntity("testing.SharedTestRunService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.SharedTestRuns.Table, executor),
			testingpb.SharedTestRunConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}

// Create mints a new shared link for testRunID, valid for ttl (0 = no expiry).
func (s *SharedTestRunService) Create(
	ctx context.Context,
	tenantID *iampb.TenantId,
	callerID *iampb.UserId,
	testRunID *testingpb.TestRunId,
	ttl time.Duration,
) (*testingpb.SharedTestRun, error) {
	return pgtx.WithSerializableRet(ctx, s.txMgr,
		func(ctx context.Context) (*testingpb.SharedTestRun, error) {
			token, err := newToken()
			if err != nil {
				return nil, err
			}

			now := timestamppb.Now()
			shared := &testingpb.SharedTestRun{
				Id:         &testingpb.SharedTestRunId{Value: ids.New()},
				TenantId:   tenantID,
				Timestamps: &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now},
				TestRunId:  testRunID,
				CreatedBy:  callerID,
				Token:      token,
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
				testingpb.SharedTestRuns.Insert().From(scanner.AllSetters()...),
			); err != nil {
				return nil, err
			}
			return shared, nil
		})
}

// Revoke soft-deletes a SharedTestRun by setting deleted_at.
func (s *SharedTestRunService) Revoke(
	ctx context.Context,
	id *testingpb.SharedTestRunId,
) (*testingpb.SharedTestRun, error) {
	existing, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	n, err := s.repo.Execute(ctx,
		testingpb.SharedTestRuns.Update().
			Set(testingpb.SharedTestRuns.DeletedAt.Set(&now)).
			Where(
				testingpb.SharedTestRuns.Id.Eq(id.GetValue()),
				testingpb.SharedTestRuns.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_test_run", id.GetValue()))
	}
	return existing, nil
}

// GetByToken looks up a non-deleted, non-expired SharedTestRun by token.
func (s *SharedTestRunService) GetByToken(
	ctx context.Context,
	token string,
) (*testingpb.SharedTestRun, error) {
	// We fetch by token and filter expired/deleted in application code because
	// the ratel query builder doesn't expose a portable "now()" comparator.
	p, err := s.repo.QueryRow(ctx,
		testingpb.SharedTestRuns.Select(safeSelectSharedRunCols...).Where(
			testingpb.SharedTestRuns.Token.Eq(token),
			testingpb.SharedTestRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_test_run", token))
		}
		return nil, err
	}
	// Check expiry.
	if ea := p.GetExpiresAt(); ea != nil {
		if ea.AsTime().Before(time.Now()) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_test_run", token))
		}
	}
	return p, nil
}

// ListByTenant returns all non-deleted SharedTestRuns for tenantID.
func (s *SharedTestRunService) ListByTenant(
	ctx context.Context,
	tenantID *iampb.TenantId,
) ([]*testingpb.SharedTestRun, error) {
	return s.repo.Query(ctx,
		testingpb.SharedTestRuns.Select(safeSelectSharedRunCols...).Where(
			testingpb.SharedTestRuns.TenantId.Eq(tenantID.GetValue()),
			testingpb.SharedTestRuns.DeletedAt.IsNull(),
		),
	)
}

func (s *SharedTestRunService) getByID(
	ctx context.Context,
	id *testingpb.SharedTestRunId,
) (*testingpb.SharedTestRun, error) {
	p, err := s.repo.QueryRow(ctx,
		testingpb.SharedTestRuns.Select(safeSelectSharedRunCols...).Where(
			testingpb.SharedTestRuns.Id.Eq(id.GetValue()),
			testingpb.SharedTestRuns.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("shared_test_run", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

// newToken generates a URL-safe 24-byte random slug.
func newToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
