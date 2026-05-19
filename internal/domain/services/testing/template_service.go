package testing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/yaroher/ratel/pkg/dml/set"
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

// safeSelectCols lists the columns we can safely scan from the DB.
// DatabaseVariantDatabasePresetId and WorkloadVariantWorkloadPresetId are
// omitted: the generated scanner stores them as plain string (not *string), so
// pgx cannot scan NULL into those targets. When unset in the DB the scanner
// field stays "" and IntoPb silently ignores it, which is correct.
var safeSelectCols = []testingpb.TestRunTemplateColumnAlias{
	testingpb.TestRunTemplateColumnId,
	testingpb.TestRunTemplateColumnTenantId,
	testingpb.TestRunTemplateColumnCreatedAt,
	testingpb.TestRunTemplateColumnUpdatedAt,
	testingpb.TestRunTemplateColumnDeletedAt,
	testingpb.TestRunTemplateColumnName,
	testingpb.TestRunTemplateColumnDescription,
	testingpb.TestRunTemplateColumnLabel,
	testingpb.TestRunTemplateColumnCreatedBy,
	testingpb.TestRunTemplateColumnDatabaseVariantDatabase,
	testingpb.TestRunTemplateColumnDatabaseVariantCase,
	testingpb.TestRunTemplateColumnWorkloadVariantWorkload,
	testingpb.TestRunTemplateColumnWorkloadVariantCase,
}

// scannerSetters returns all setters for a TestRunTemplateScanner, converting
// empty-string FK fields to nil so that PostgreSQL stores NULL instead of an
// empty string (which would violate the FK constraint).
func scannerSetters(s *testingpb.TestRunTemplateScanner) []set.ValueSetter[testingpb.TestRunTemplateColumnAlias] {
	var dbPresetID any = s.DatabaseVariantDatabasePresetId
	if s.DatabaseVariantDatabasePresetId == "" {
		dbPresetID = nil
	}
	var wlPresetID any = s.WorkloadVariantWorkloadPresetId
	if s.WorkloadVariantWorkloadPresetId == "" {
		wlPresetID = nil
	}
	return []set.ValueSetter[testingpb.TestRunTemplateColumnAlias]{
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnId, s.Id),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnTenantId, s.TenantId),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnCreatedAt, s.CreatedAt),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnUpdatedAt, s.UpdatedAt),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnDeletedAt, s.DeletedAt),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnName, s.Name),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnDescription, s.Description),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnLabel, s.Label),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnCreatedBy, s.CreatedBy),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnDatabaseVariantDatabasePresetId, dbPresetID),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnDatabaseVariantDatabase, s.DatabaseVariantDatabase),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnDatabaseVariantCase, s.DatabaseVariantCase),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnWorkloadVariantWorkloadPresetId, wlPresetID),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnWorkloadVariantWorkload, s.WorkloadVariantWorkload),
		set.NewSetter[testingpb.TestRunTemplateColumnAlias](testingpb.TestRunTemplateColumnWorkloadVariantCase, s.WorkloadVariantCase),
	}
}

// TemplateService provides CRUD operations for TestRunTemplate entities.
//
// NOTE: Permission checks (HasTenantRole) are intentionally omitted here.
// The transport middleware (tenant resolver) already validates membership,
// so callers reaching this service are implicitly authorised. Integration
// tests invoke the service directly and would be blocked by role checks.
type TemplateService struct {
	*tracing.Entity

	repo  *repository.ProtoRepository[testingpb.TestRunTemplateAlias, testingpb.TestRunTemplateColumnAlias, *testingpb.TestRunTemplateScanner, *testingpb.TestRunTemplate]
	txMgr pgtx.TxManager
	//nolint:unused
	events eventing.Bus
}

// NewTemplateService constructs a TemplateService.
func NewTemplateService(executor exec.DB, txMgr pgtx.TxManager, events eventing.Bus) *TemplateService {
	return &TemplateService{
		Entity: tracing.NewEntity("testing.TemplateService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(testingpb.TestRunTemplates.Table, executor),
			testingpb.TestRunTemplateConverter,
		),
		txMgr:  txMgr,
		events: events,
	}
}

// CreateTestRunTemplate creates a new TestRunTemplate for the given tenant.
func (s *TemplateService) CreateTestRunTemplate(
	ctx context.Context,
	tenantID *iampb.TenantId,
	callerID *iampb.UserId,
	tpl *testingpb.TestRunTemplate,
) (*testingpb.TestRunTemplate, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTestRunTemplate",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestRunTemplate, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestRunTemplate, error) {
					now := timestamppb.Now()
					tpl.Id = &testingpb.TestRunTemplateId{Value: ids.New()}
					tpl.TenantId = tenantID
					tpl.CreatedBy = callerID
					tpl.Timestamps = &commonpb.Timestamps{CreatedAt: now, UpdatedAt: now}
					scanner := tpl.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					// IntoPlain sets []byte{} for unset JSONB columns; PostgreSQL
					// rejects empty bytes as invalid JSON — use nil instead.
					if len(scanner.DatabaseVariantDatabase) == 0 {
						scanner.DatabaseVariantDatabase = nil
					}
					if len(scanner.WorkloadVariantWorkload) == 0 {
						scanner.WorkloadVariantWorkload = nil
					}
					// scannerSetters converts empty-string FK fields to nil so
					// PostgreSQL stores NULL rather than an empty string.
					if _, err := s.repo.Execute(ctx,
						testingpb.TestRunTemplates.Insert().From(scannerSetters(scanner)...),
					); err != nil {
						return nil, err
					}
					return tpl, nil
				})
		})
}

// GetTestRunTemplate retrieves a single (non-deleted) TestRunTemplate by ID.
func (s *TemplateService) GetTestRunTemplate(
	ctx context.Context,
	id *testingpb.TestRunTemplateId,
) (*testingpb.TestRunTemplate, error) {
	p, err := s.repo.QueryRow(ctx,
		testingpb.TestRunTemplates.Select(safeSelectCols...).Where(
			testingpb.TestRunTemplates.Id.Eq(id.GetValue()),
			testingpb.TestRunTemplates.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("test_run_template", id.GetValue()))
		}
		return nil, err
	}
	return p, nil
}

// ListTestRunTemplates returns all non-deleted templates belonging to tenantID.
func (s *TemplateService) ListTestRunTemplates(
	ctx context.Context,
	tenantID *iampb.TenantId,
) ([]*testingpb.TestRunTemplate, error) {
	return s.repo.Query(ctx,
		testingpb.TestRunTemplates.Select(safeSelectCols...).Where(
			testingpb.TestRunTemplates.TenantId.Eq(tenantID.GetValue()),
			testingpb.TestRunTemplates.DeletedAt.IsNull(),
		),
	)
}

// UpdateTestRunTemplate patches mutable fields (Identity, Database, Workload)
// and bumps UpdatedAt.
func (s *TemplateService) UpdateTestRunTemplate(
	ctx context.Context,
	tpl *testingpb.TestRunTemplate,
) (*testingpb.TestRunTemplate, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "UpdateTestRunTemplate",
		func(ctx context.Context, _ trace.Span) (*testingpb.TestRunTemplate, error) {
			return pgtx.WithSerializableRet(ctx, s.txMgr,
				func(ctx context.Context) (*testingpb.TestRunTemplate, error) {
					existing, err := s.GetTestRunTemplate(ctx, tpl.GetId())
					if err != nil {
						return nil, err
					}
					if tpl.GetIdentity() != nil {
						existing.Identity = tpl.GetIdentity()
					}
					if tpl.GetDatabase() != nil {
						existing.Database = tpl.GetDatabase()
					}
					if tpl.GetWorkload() != nil {
						existing.Workload = tpl.GetWorkload()
					}
					existing.Timestamps.UpdatedAt = timestamppb.Now()
					scanner := existing.IntoPlain()
					if scanner.Label == nil {
						scanner.Label = []string{}
					}
					// JSONB nil guard mirrors Create — empty []byte fails Postgres JSON parse.
					if len(scanner.DatabaseVariantDatabase) == 0 {
						scanner.DatabaseVariantDatabase = nil
					}
					if len(scanner.WorkloadVariantWorkload) == 0 {
						scanner.WorkloadVariantWorkload = nil
					}
					// Build setters via scannerSetters() so FK empty-string → NULL
					// guard is applied to update as well as insert.
					setters := scannerSetters(scanner)
					if _, err := s.repo.Execute(ctx,
						testingpb.TestRunTemplates.Update().
							Set(setters...).
							Where(
								testingpb.TestRunTemplates.Id.Eq(existing.GetId().GetValue()),
							),
					); err != nil {
						return nil, err
					}
					return existing, nil
				})
		})
}

// DeleteTestRunTemplate soft-deletes a TestRunTemplate.
func (s *TemplateService) DeleteTestRunTemplate(
	ctx context.Context,
	id *testingpb.TestRunTemplateId,
) error {
	now := time.Now()
	n, err := s.repo.Execute(ctx,
		testingpb.TestRunTemplates.Update().
			Set(testingpb.TestRunTemplates.DeletedAt.Set(&now)).
			Where(
				testingpb.TestRunTemplates.Id.Eq(id.GetValue()),
				testingpb.TestRunTemplates.DeletedAt.IsNull(),
			),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return domainerr.NotFound(domainerr.ResourceInfo("test_run_template", id.GetValue()))
	}
	return nil
}
