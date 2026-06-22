package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	domsettings "github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/adapters"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/execution"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/identity"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	apipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/suite"
	testwizardsvc "github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
)

// unmarshalJSON decodes a canonical protojson blob (the encoding every postgres
// record column uses) into m, mapping pgx.ErrNoRows onto the domain not-found.
var unmarshalJSON = protojson.UnmarshalOptions{DiscardUnknown: true}

/*
	===== by-id record getters =====

	Several adapter consumer interfaces load a record by id alone (the service has
	already authorised the caller), but the postgres TestRunRepo.Get is scoped to
	(tenant_id, id). These thin getters query the same jsonb-blob tables by id
	directly through the ambient-transaction executor, mirroring the repo's codec.
*/

// byIDReader resolves test/suite run records by id alone over db.TxDB.
type byIDReader struct{ db *postgres.DB }

func (r byIDReader) testRun(ctx context.Context, id string) (*modelspb.TestRunRecord, error) {
	var data []byte
	err := r.db.TxDB.QueryRow(ctx,
		`select data from test_run_records where id = $1`, id).Scan(&data)
	if err != nil {
		return nil, translate("test_run", err)
	}
	rec := &modelspb.TestRunRecord{}
	if err := unmarshalJSON.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r byIDReader) suiteRun(ctx context.Context, id string) (*modelspb.SuiteRunRecord, error) {
	var data []byte
	err := r.db.TxDB.QueryRow(ctx,
		`select data from suite_run_records where id = $1`, id).Scan(&data)
	if err != nil {
		return nil, translate("suite_run", err)
	}
	rec := &modelspb.SuiteRunRecord{}
	if err := unmarshalJSON.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func uuidString() string { return uuid.NewString() }

func timestamppbNow() *timestamppb.Timestamp { return timestamppb.New(time.Now()) }

func translate(resource string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return derrors.NotFound(resource, "not found")
	}
	return err
}

// runRecordGetter backs compare.RunRecordGetter / favorite TestRuns getter:
// Get(ctx, id) -> *TestRunRecord.
type runRecordGetter struct{ r byIDReader }

func (g runRecordGetter) Get(ctx context.Context, id string) (*modelspb.TestRunRecord, error) {
	return g.r.testRun(ctx, id)
}

// shareRunReader backs adapters.ShareRunReader (GetTestRun/GetSuiteRun by id).
type shareRunReader struct {
	r         byIDReader
	suiteRuns *postgres.SuiteRunRepo
}

func (s shareRunReader) GetTestRun(ctx context.Context, id string) (*modelspb.TestRunRecord, error) {
	return s.r.testRun(ctx, id)
}

func (s shareRunReader) GetSuiteRun(ctx context.Context, id string) (*modelspb.SuiteRunRecord, error) {
	return s.suiteRuns.Get(ctx, id)
}

// snapshotRunReader backs execution.SnapshotRunReader: RunRecord by id + SuiteRun
// by id (nil when the run is standalone).
type snapshotRunReader struct {
	r         byIDReader
	suiteRuns *postgres.SuiteRunRepo
}

var _ execution.SnapshotRunReader = snapshotRunReader{}

func (s snapshotRunReader) RunRecord(ctx context.Context, runID string) (*modelspb.TestRunRecord, error) {
	return s.r.testRun(ctx, runID)
}

func (s snapshotRunReader) SuiteRun(ctx context.Context, suiteRunID string) (*modelspb.SuiteRunRecord, error) {
	if suiteRunID == "" {
		return nil, nil
	}
	return s.suiteRuns.Get(ctx, suiteRunID)
}

// runtimePersistenceStore backs workflow runtime persistence activities. Reads are
// by id because workflows know the run id, then writes go through the typed repos
// using the tenant carried in each record.
type runtimePersistenceStore struct {
	r         byIDReader
	runs      *postgres.TestRunRepo
	suiteRuns *postgres.SuiteRunRepo
	suites    *postgres.SuiteRepo
}

var _ execution.RunPersistenceStore = runtimePersistenceStore{}

func (s runtimePersistenceStore) RunRecord(ctx context.Context, runID string) (*modelspb.TestRunRecord, error) {
	return s.r.testRun(ctx, runID)
}

func (s runtimePersistenceStore) SaveRunRecord(ctx context.Context, run *modelspb.TestRunRecord) error {
	return s.runs.Update(ctx, run)
}

func (s runtimePersistenceStore) SuiteRun(ctx context.Context, suiteRunID string) (*modelspb.SuiteRunRecord, error) {
	return s.suiteRuns.Get(ctx, suiteRunID)
}

func (s runtimePersistenceStore) SaveSuiteRun(ctx context.Context, suiteRun *modelspb.SuiteRunRecord) error {
	return s.suiteRuns.Update(ctx, suiteRun)
}

func (s runtimePersistenceStore) SuiteRecord(ctx context.Context, tenantID, suiteID string) (*modelspb.SuiteRecord, error) {
	return s.suites.Get(ctx, tenantID, suiteID)
}

func (s runtimePersistenceStore) SaveSuiteRecord(ctx context.Context, suite *modelspb.SuiteRecord) error {
	return s.suites.Update(ctx, suite)
}

/*
	===== caller source =====

	execution.CallerSource resolves the acting account id from the authenticated
	claims the upstream interceptor (or the service) stashed in ctx.
*/

type authnCaller struct{ authn *identity.JWTAuthn }

var _ execution.CallerSource = authnCaller{}

func (c authnCaller) AccountID(ctx context.Context) (string, error) {
	claims, err := c.authn.Caller(ctx)
	if err != nil {
		return "", err
	}
	return claims.GetAccountId(), nil
}

/*
	===== settings resolver sources =====

	The domain settings.Resolver reads the platform singleton + per-tenant settings
	through narrow source interfaces. The identity.SettingsReader already adapts the
	platform getter (treating a brand-new install as zero-value); the tenant source
	maps the postgres TenantSettingsRepo.Get onto TenantSettings.
*/

type tenantSettingsSource struct{ repo *postgres.TenantSettingsRepo }

var _ domsettings.TenantSettingsSource = tenantSettingsSource{}

func (s tenantSettingsSource) TenantSettings(ctx context.Context, tenantID string) (*modelspb.TenantSettingsRecord, error) {
	return s.repo.Get(ctx, tenantID)
}

/*
	===== rating runs lister =====

	The rating boards read candidate runs flagged for a scope. Tenant scope reuses
	the tenant-scoped repo List; global scope crosses tenants, so it queries the
	rating-flagged completed runs directly over db.TxDB.
*/

type ratingRunsLister struct {
	db   *postgres.DB
	runs *postgres.TestRunRepo
}

var _ adapters.RatingRunsLister = ratingRunsLister{}

func (l ratingRunsLister) ListRatingRuns(ctx context.Context, scope adapters.RatingRunsScope, tenantID string) ([]*modelspb.TestRunRecord, error) {
	switch scope {
	case adapters.RatingScopeTenant:
		all, _, err := l.runs.List(ctx, &apipb.ListTestRunsRequest{TenantId: tenantID}, "")
		if err != nil {
			return nil, err
		}
		out := all[:0]
		for _, r := range all {
			if r.GetInTenantRating() && r.GetStatus() == commonpb.Status_STATUS_COMPLETED {
				out = append(out, r)
			}
		}
		return out, nil
	default: // global, cross-tenant
		rows, err := l.db.TxDB.Query(ctx, `select data from test_run_records where data->'entity'->'timings'->>'deletedAt' is null`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]*modelspb.TestRunRecord, 0)
		for rows.Next() {
			var data []byte
			if err := rows.Scan(&data); err != nil {
				return nil, err
			}
			rec := &modelspb.TestRunRecord{}
			if err := unmarshalJSON.Unmarshal(data, rec); err != nil {
				return nil, err
			}
			if rec.GetInGlobalRating() && rec.GetStatus() == commonpb.Status_STATUS_COMPLETED {
				out = append(out, rec)
			}
		}
		return out, rows.Err()
	}
}

/*
	===== rating name resolver =====
*/

type ratingNames struct {
	accounts *postgres.AccountRepo
	tenants  *postgres.TenantRepo
}

var _ adapters.RatingNameResolver = ratingNames{}

func (n ratingNames) AccountName(ctx context.Context, accountID string) string {
	acc, err := n.accounts.Get(ctx, accountID)
	if err != nil {
		return ""
	}
	if acc.GetNickname() != "" {
		return acc.GetNickname()
	}
	return acc.GetEmail()
}

func (n ratingNames) TenantName(ctx context.Context, tenantID string) string {
	t, err := n.tenants.Get(ctx, tenantID)
	if err != nil {
		return ""
	}
	return t.GetName()
}

/*
	===== tenant membership / machine resolvers (agent shell) =====
*/

type tenantMembership struct{ memberships *postgres.MembershipRepo }

var _ adapters.TenantMembershipChecker = tenantMembership{}

func (m tenantMembership) IsMember(ctx context.Context, accountID, tenantID string) (bool, error) {
	_, err := m.memberships.GetByAccountTenant(ctx, accountID, tenantID)
	if err != nil {
		// An absent membership (pgx no-rows mapped to the domain not-found) simply
		// means the account is not a member of the tenant.
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, derrors.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// allMachinesResolver treats every machine as registered (the in-memory hub
// enforces liveness; there is no separate machine registry table). It never
// rejects, so the locator falls through to the hub's live-stream check.
type allMachinesResolver struct{}

var _ adapters.MachineAddressResolver = allMachinesResolver{}

func (allMachinesResolver) Lookup(context.Context, string, string, string) error { return nil }

/*
	===== dashboard runs reader =====
*/

type dashboardRuns struct {
	runs      *postgres.TestRunRepo
	suiteRuns *postgres.SuiteRunRepo
	suites    *postgres.SuiteRepo
}

var _ adapters.DashboardRunsReader = dashboardRuns{}

func (d dashboardRuns) ListTenantRuns(ctx context.Context, tenantID string) ([]*modelspb.TestRunRecord, error) {
	out, _, err := d.runs.List(ctx, &apipb.ListTestRunsRequest{TenantId: tenantID}, "")
	return out, err
}

func (d dashboardRuns) ListTenantSuiteRuns(ctx context.Context, tenantID string) ([]*modelspb.SuiteRunRecord, error) {
	out, _, err := d.suiteRuns.List(ctx, &apipb.ListSuiteRunsRequest{TenantId: tenantID}, "")
	return out, err
}

func (d dashboardRuns) ListScheduledSuites(ctx context.Context, tenantID string) ([]*modelspb.SuiteRecord, error) {
	all, _, err := d.suites.List(ctx, suiteListQuery(tenantID))
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, s := range all {
		if s.GetSpec().GetSchedule().GetEnabled() {
			out = append(out, s)
		}
	}
	return out, nil
}

/*
	===== suite cell resolver (execution + adapters) =====

	Resolves a suite cell source (preset pair / test preset / inline) into a
	db+workload domain.Test over the postgres preset repos. Backs both
	execution.CellResolver and adapters.SuiteCellResolver.
*/

type cellResolver struct {
	dbPresets       *postgres.DatabasePresetRepo
	workloadPresets *postgres.WorkloadPresetRepo
	testPresets     *postgres.TestPresetRepo
}

var _ execution.CellResolver = cellResolver{}

func (c cellResolver) ResolveCell(ctx context.Context, tenantID string, cell *domain.SuiteCell) (*domain.Test, error) {
	switch src := cell.GetSource().(type) {
	case *domain.SuiteCell_InlineTest:
		return src.InlineTest, nil
	case *domain.SuiteCell_TestPresetId:
		return c.TestPreset(ctx, tenantID, src.TestPresetId)
	case *domain.SuiteCell_PresetPair_:
		db, err := c.DatabasePreset(ctx, tenantID, src.PresetPair.GetDbPresetId())
		if err != nil {
			return nil, err
		}
		wl, err := c.WorkloadPreset(ctx, tenantID, src.PresetPair.GetWorkloadPresetId())
		if err != nil {
			return nil, err
		}
		return &domain.Test{Database: db, Workload: wl}, nil
	default:
		return nil, derrors.Invalid("cell.source", "suite cell has no source")
	}
}

func (c cellResolver) DatabasePreset(ctx context.Context, tenantID, presetID string) (*domain.Database, error) {
	rec, err := c.dbPresets.Get(ctx, tenantID, presetID, "")
	if err != nil {
		return nil, err
	}
	return rec.GetDatabase(), nil
}

func (c cellResolver) WorkloadPreset(ctx context.Context, tenantID, presetID string) (*domain.Workload, error) {
	rec, err := c.workloadPresets.Get(ctx, tenantID, presetID, "")
	if err != nil {
		return nil, err
	}
	return rec.GetWorkload(), nil
}

func (c cellResolver) TestPreset(ctx context.Context, tenantID, presetID string) (*domain.Test, error) {
	rec, err := c.testPresets.Get(ctx, tenantID, presetID, "")
	if err != nil {
		return nil, err
	}
	return rec.GetTest(), nil
}

/*
	===== suite baker (suite_wizard) =====

	The suite wizard's Finish path persists the reusable SuiteRecord and, on
	start=true, prepares a SuiteRunRecord plus post-commit starter. Preparation
	reuses the shared execution.SuiteRunLauncher, which deterministically
	re-expands the suite spec (the same children the wizard baked), so the
	pre-baked children list is not re-persisted here.
*/

type suiteBaker struct {
	suites    *postgres.SuiteRepo
	suiteRuns *postgres.SuiteRunRepo
	launcher  *execution.SuiteRunLauncher
	caller    execution.CallerSource
}

var _ adapters.SuiteBaker = (*suiteBaker)(nil)

func (b *suiteBaker) SaveSuite(ctx context.Context, suiteRec *modelspb.SuiteRecord, replaceSuiteID string) (*modelspb.SuiteRecord, error) {
	if replaceSuiteID != "" {
		existing, err := b.suites.Get(ctx, suiteRec.GetEntity().GetTenantId(), replaceSuiteID)
		if err != nil {
			return nil, err
		}
		if existing.GetEntity().GetTimings().GetDeletedAt() != nil {
			return nil, derrors.FailedPrecondition("suite_deleted", "suite is deleted")
		}
		suiteRec.Entity.Id = existing.GetEntity().GetId()
		suiteRec.Entity.TenantId = existing.GetEntity().GetTenantId()
		suiteRec.Entity.AuthorId = existing.GetEntity().GetAuthorId()
		suiteRec.Entity.Description = existing.GetEntity().GetDescription()
		suiteRec.Entity.Timings = &commonpb.Timings{
			CreatedAt: existing.GetEntity().GetTimings().GetCreatedAt(),
			UpdatedAt: timestamppbNow(),
			DeletedAt: existing.GetEntity().GetTimings().GetDeletedAt(),
		}
		suiteRec.GetSpec().Id = existing.GetEntity().GetId()
		suiteRec.Summary = suiteSummaryFromSpec(suiteRec.GetSpec(), existing.GetSummary())
		if err := b.suites.Update(ctx, suiteRec); err != nil {
			return nil, err
		}
		return suiteRec, nil
	}
	if err := b.suites.Create(ctx, suiteRec); err != nil {
		return nil, err
	}
	return suiteRec, nil
}

func (b *suiteBaker) StartSuiteRun(
	ctx context.Context,
	suiteRec *modelspb.SuiteRecord,
	_ []*adapters.BakedSuiteChild,
	trigger commonpb.Trigger,
	maxParallel uint32,
) (*modelspb.SuiteRunRecord, func(context.Context) error, error) {
	authorID := suiteRec.GetEntity().GetAuthorId()
	if authorID == "" && b.caller != nil {
		if id, err := b.caller.AccountID(ctx); err == nil {
			authorID = id
		}
	}
	now := timestamppbNow()
	run := &modelspb.SuiteRunRecord{
		Entity: &commonpb.Entity{
			Id:       uuidString(),
			TenantId: suiteRec.GetEntity().GetTenantId(),
			Name:     suiteRec.GetEntity().GetName(),
			AuthorId: authorID,
			Timings:  &commonpb.Timings{CreatedAt: now, UpdatedAt: now},
		},
		SuiteId:     suiteRec.GetEntity().GetId(),
		Status:      commonpb.Status_STATUS_PENDING,
		Trigger:     trigger,
		MaxParallel: maxParallel,
	}
	// Launch expands children, persists the parent, and returns the post-commit
	// SuiteWorkflow starter.
	starter, err := b.launcher.Launch(ctx, run, suiteRec.GetSpec())
	if err != nil {
		return nil, nil, err
	}
	if suiteRec.Summary == nil {
		suiteRec.Summary = &modelspb.SuiteRecord_Summary{}
	}
	suiteRec.Summary.RunCount++
	suiteRec.Summary.LastRunAt = now
	suiteRec.Summary.LastRunStatus = run.GetStatus()
	if suiteRec.Entity != nil {
		if suiteRec.Entity.Timings == nil {
			suiteRec.Entity.Timings = &commonpb.Timings{}
		}
		suiteRec.Entity.Timings.UpdatedAt = now
	}
	if err := b.suites.Update(ctx, suiteRec); err != nil {
		return nil, nil, err
	}
	return run, starter, nil
}

// suiteListQuery builds a tenant-scoped suite list query for the dashboard's
// scheduled-suites read (filter/sort/paging defaulted; the adapter filters to
// schedule-enabled suites in memory).
func suiteListQuery(tenantID string) suite.SuiteListQuery {
	return suite.SuiteListQuery{TenantID: tenantID}
}

/*
	===== child run persister =====

	execution.ChildRunPersister wants CreateChildRun; the postgres TestRunRepo
	exposes Create. This thin adapter bridges the method name.
*/

type childRunPersister struct{ runs *postgres.TestRunRepo }

var _ execution.ChildRunPersister = childRunPersister{}

func (p childRunPersister) CreateChildRun(ctx context.Context, run *modelspb.TestRunRecord) error {
	return p.runs.Create(ctx, run)
}

func (p childRunPersister) UpdateChildRun(ctx context.Context, run *modelspb.TestRunRecord) error {
	return p.runs.Update(ctx, run)
}

type suiteRunPersister struct{ suiteRuns *postgres.SuiteRunRepo }

var _ execution.SuiteRunPersister = suiteRunPersister{}

func (p suiteRunPersister) SaveSuiteRun(ctx context.Context, run *modelspb.SuiteRunRecord) error {
	return p.suiteRuns.Update(ctx, run)
}

func suiteSummaryFromSpec(spec *domain.Suite, prev *modelspb.SuiteRecord_Summary) *modelspb.SuiteRecord_Summary {
	out := &modelspb.SuiteRecord_Summary{}
	if prev != nil {
		out.RunCount = prev.GetRunCount()
		out.LastRunAt = prev.GetLastRunAt()
		out.LastRunStatus = prev.GetLastRunStatus()
		out.NextRunAt = prev.GetNextRunAt()
	}
	out.CellCount = enabledSuiteCellCount(spec)
	if sched := spec.GetSchedule(); sched != nil {
		out.ScheduleEnabled = sched.GetEnabled()
		out.Cron = sched.GetCron()
		if !sched.GetEnabled() {
			out.NextRunAt = nil
		}
		return out
	}
	out.ScheduleEnabled = false
	out.Cron = ""
	out.NextRunAt = nil
	return out
}

func enabledSuiteCellCount(spec *domain.Suite) uint32 {
	var count uint32
	for _, cell := range spec.GetCells() {
		if cell.GetEnabled() {
			count++
		}
	}
	return count
}

/*
	===== suite reader (suite_wizard) =====

	suite_wizard.SuiteReader.Get returns the suite SPEC (*domain.Suite); the
	postgres SuiteRepo.Get returns the stored record. This adapter projects the
	record's spec.
*/

type suiteSpecReader struct{ suites *postgres.SuiteRepo }

func (r suiteSpecReader) Get(ctx context.Context, tenantID, id string) (*domain.Suite, error) {
	rec, err := r.suites.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return rec.GetSpec(), nil
}

/*
	===== stroppy prober (test_wizard.ProbeScript) =====

	The test_wizard service declares a cycle-free Prober port (it must not import
	internal/infrastructure/execution). This adapter satisfies that port by mapping
	the port's field struct onto execution.StroppyProber, which owns the actual
	`stroppy probe` exec + binary fetch/cache.
*/

type stroppyProberAdapter struct{ prober *execution.StroppyProber }

var _ testwizardsvc.Prober = stroppyProberAdapter{}

func (a stroppyProberAdapter) Probe(ctx context.Context, in testwizardsvc.ProbeInput) (*testwizardsvc.ProbeOutput, error) {
	files := make([]execution.ProbeWorkloadFile, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, execution.ProbeWorkloadFile{Name: f.Name, Content: f.Content})
	}
	res, err := a.prober.Probe(ctx, execution.ProbeRequest{
		Version:     in.Version,
		Script:      in.Script,
		SQL:         in.SQL,
		DriverType:  in.DriverType,
		PoolSize:    in.PoolSize,
		ScaleFactor: in.ScaleFactor,
		Env:         in.Env,
		Files:       files,
	}, execution.ProbeOptions{IncludeHuman: in.IncludeHuman})
	if err != nil {
		return nil, err
	}
	return &testwizardsvc.ProbeOutput{Metadata: res.Metadata, Human: res.Human}, nil
}

func (a stroppyProberAdapter) Catalog(ctx context.Context, version string) ([]string, error) {
	return a.prober.Catalog(ctx, version)
}
