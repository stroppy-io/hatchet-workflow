package testing

import (
	"context"
	"errors"
	"time"

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

// safeSelectSuiteCols lists the columns we can safely scan from the DB.
// Matrix and Policy are stored as JSONB text columns (non-nullable with defaults),
// so all columns here are safe to include.
var safeSelectSuiteCols = []testingpb.TestSuiteColumnAlias{
	testingpb.TestSuiteColumnId,
	testingpb.TestSuiteColumnTenantId,
	testingpb.TestSuiteColumnCreatedAt,
	testingpb.TestSuiteColumnUpdatedAt,
	testingpb.TestSuiteColumnDeletedAt,
	testingpb.TestSuiteColumnName,
	testingpb.TestSuiteColumnDescription,
	testingpb.TestSuiteColumnLabel,
	testingpb.TestSuiteColumnCreatedBy,
	testingpb.TestSuiteColumnMatrix,
	testingpb.TestSuiteColumnPolicy,
}

// TestSuiteService provides CRUD + Clone operations for TestSuite entities.
//
// NOTE: Permission checks (HasTenantRole) are intentionally omitted here.
// The transport middleware (tenant resolver) already validates membership,
// so callers reaching this service are implicitly authorised.
type TestSuiteService struct {
	*tracing.Entity

	repo  *repository.ProtoRepository[testingpb.TestSuiteAlias, testingpb.TestSuiteColumnAlias, *testingpb.TestSuiteScanner, *testingpb.TestSuite]
	txMgr pgtx.TxManager
	//nolint:unused
	events eventing.Bus
}

// NewTestSuiteService constructs a TestSuiteService.
func NewTestSuiteService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *TestSuiteService {
	return &TestSuiteService{
		Entity: tracing.NewEntity("testing.TestSuiteService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestSuites.Table, executor),
			testingpb.TestSuiteConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}

// CreateTestSuite creates a new TestSuite for the given tenant.
func (s *TestSuiteService) CreateTestSuite(
	ctx context.Context,
	tenantID *iampb.TenantId,
	callerID *iampb.UserId,
	suite *testingpb.TestSuite,
) (*testingpb.TestSuite, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTestSuite",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestSuite, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestSuite, error) {
					now := timestamppb.Now()
					suite.Id = &testingpb.TestSuiteId{Value: ids.New()}
					suite.TenantId = tenantID
					suite.CreatedBy = callerID
					suite.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
					scanner := suite.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					// IntoPlain sets []byte{} for unset JSONB columns; PostgreSQL
					// rejects empty bytes as invalid JSON — use the empty-object literal
					// so the NOT NULL constraint is satisfied.
					if len(scanner.Matrix) == 0 {
						scanner.Matrix = []byte("{}")
					}
					if len(scanner.Policy) == 0 {
						scanner.Policy = []byte("{}")
					}
					if _, err := s.repo.Execute(ctx,
						testingpb.TestSuites.Insert().From(scanner.AllSetters()...),
					); err != nil {
						return nil, err
					}
					return suite, nil
				})
		})
}

// GetTestSuite retrieves a single (non-deleted) TestSuite by ID.
func (s *TestSuiteService) GetTestSuite(
	ctx context.Context,
	id *testingpb.TestSuiteId,
) (*testingpb.TestSuite, error) {
	p, err := s.repo.QueryRow(ctx,
		testingpb.TestSuites.Select(safeSelectSuiteCols...).Where(
			testingpb.TestSuites.Id.Eq(id.GetValue()),
			testingpb.TestSuites.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("test_suite", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

// ListTestSuites returns all non-deleted test suites belonging to tenantID.
func (s *TestSuiteService) ListTestSuites(
	ctx context.Context,
	tenantID *iampb.TenantId,
) ([]*testingpb.TestSuite, error) {
	return s.repo.Query(ctx,
		testingpb.TestSuites.Select(safeSelectSuiteCols...).Where(
			testingpb.TestSuites.TenantId.Eq(tenantID.GetValue()),
			testingpb.TestSuites.DeletedAt.IsNull(),
		),
	)
}

// UpdateTestSuite patches mutable fields (Identity, Matrix, Policy) and bumps UpdatedAt.
func (s *TestSuiteService) UpdateTestSuite(
	ctx context.Context,
	suite *testingpb.TestSuite,
) (*testingpb.TestSuite, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "UpdateTestSuite",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestSuite, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestSuite, error) {
					existing, err := s.GetTestSuite(ctx, suite.GetId())
					if err != nil {
						return nil, err
					}
					if suite.GetIdentity() != nil {
						existing.Identity = suite.GetIdentity()
					}
					if suite.GetMatrix() != nil {
						existing.Matrix = suite.GetMatrix()
					}
					if suite.GetPolicy() != nil {
						existing.Policy = suite.GetPolicy()
					}
					existing.Timestamps.UpdatedAt = timestamppb.Now()
					scanner := existing.IntoPlain()
					if _, err := s.repo.Execute(ctx,
						testingpb.TestSuites.Update().
							Set(
								scanner.GetSetter(testingpb.TestSuiteColumnName)(),
								scanner.GetSetter(testingpb.TestSuiteColumnDescription)(),
								scanner.GetSetter(testingpb.TestSuiteColumnLabel)(),
								scanner.GetSetter(testingpb.TestSuiteColumnMatrix)(),
								scanner.GetSetter(testingpb.TestSuiteColumnPolicy)(),
								scanner.GetSetter(testingpb.TestSuiteColumnUpdatedAt)(),
							).
							Where(
								testingpb.TestSuites.Id.Eq(existing.GetId().GetValue()),
							),
					); err != nil {
						return nil, err
					}
					return existing, nil
				})
		})
}

// DeleteTestSuite soft-deletes a TestSuite and returns the pre-delete state.
func (s *TestSuiteService) DeleteTestSuite(
	ctx context.Context,
	id *testingpb.TestSuiteId,
) (*testingpb.TestSuite, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "DeleteTestSuite",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestSuite, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestSuite, error) {
					existing, err := s.GetTestSuite(ctx, id)
					if err != nil {
						return nil, err
					}
					now := time.Now()
					n, err := s.repo.Execute(ctx,
						testingpb.TestSuites.Update().
							Set(testingpb.TestSuites.DeletedAt.Set(&now)).
							Where(
								testingpb.TestSuites.Id.Eq(id.GetValue()),
								testingpb.TestSuites.DeletedAt.IsNull(),
							),
					)
					if err != nil {
						return nil, err
					}
					if n == 0 {
						return nil, domainerr.NotFound(domainerr.ResourceInfo("test_suite", id.GetValue()))
					}
					return existing, nil
				})
		})
}

// CloneTestSuite fetches the original suite and creates a new one with the
// same Identity, Matrix, and Policy but a fresh ULID.
func (s *TestSuiteService) CloneTestSuite(
	ctx context.Context,
	id *testingpb.TestSuiteId,
	callerID *iampb.UserId,
) (*testingpb.TestSuite, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CloneTestSuite",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestSuite, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestSuite, error) {
					original, err := s.GetTestSuite(ctx, id)
					if err != nil {
						return nil, err
					}
					clone := &testingpb.TestSuite{
						Identity: original.Identity,
						Matrix:   original.Matrix,
						Policy:   original.Policy,
					}
					return s.CreateTestSuite(ctx, original.GetTenantId(), callerID, clone)
				})
		})
}
