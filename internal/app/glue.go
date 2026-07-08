package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	domsettings "github.com/stroppy-io/stroppy-cloud/internal/domain/settings"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/adapters"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/execution"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	apipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// unmarshalJSON decodes a canonical protojson blob (the encoding every postgres
// record column uses) into m, mapping pgx.ErrNoRows onto the domain not-found.
var unmarshalJSON = protojson.UnmarshalOptions{DiscardUnknown: true}

/*
	===== by-id record getters =====

	Several adapter consumer interfaces load a record by id alone (the service has
	already authorised the caller), but the postgres TestRunRepo.Get is scoped to
	(tenant_id, id). This thin getter queries the same jsonb-blob table by id
	directly through the ambient-transaction executor, mirroring the repo's codec.
*/

// byIDReader resolves test run records by id alone over db.TxDB.
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

// run reads run_records by id alone, mirroring testRun above for the
// models.Run write path SP-E Task 3 cut RunRecipeWorkflow over to. Used by
// runtimePersistenceStore (workflow persist activities, which only know the
// run id) and by snapshotRunReader's fallback (see that type's doc) for a
// run that only exists in run_records (post-cutover), not test_run_records.
func (r byIDReader) run(ctx context.Context, id string) (*modelspb.Run, error) {
	var data []byte
	err := r.db.TxDB.QueryRow(ctx,
		`select data from run_records where id = $1`, id).Scan(&data)
	if err != nil {
		return nil, translate("run", err)
	}
	rec := &modelspb.Run{}
	if err := unmarshalJSON.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

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

// shareRunReader backs adapters.ShareRunReader (GetTestRun by id).
type shareRunReader struct{ r byIDReader }

func (s shareRunReader) GetTestRun(ctx context.Context, id string) (*modelspb.TestRunRecord, error) {
	return s.r.testRun(ctx, id)
}

// snapshotRunReader backs execution.SnapshotRunReader: RunRecord by id.
//
// SP-E Task 3 cut RunRecipeWorkflow's write path over to models.Run/
// run_records, but execution.OverviewReader (the sole consumer of this type)
// is not migrated to read models.Run natively until Task 4 — see
// execution.RunToTestRunRecord's doc. Until then, this reads test_run_records
// first (pre-cutover / historical runs, still there since nothing deletes
// them), falling back to run_records + the shim adapter for a run that only
// exists there (any run started after this task landed). Delete the fallback
// once Task 4 lands and OverviewReader reads *models.Run directly.
type snapshotRunReader struct{ r byIDReader }

var _ execution.SnapshotRunReader = snapshotRunReader{}

func (s snapshotRunReader) RunRecord(ctx context.Context, runID string) (*modelspb.TestRunRecord, error) {
	rec, err := s.r.testRun(ctx, runID)
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, derrors.ErrNotFound) {
		return nil, err
	}
	run, rerr := s.r.run(ctx, runID)
	if rerr != nil {
		// Neither table has this id: surface the original test_run_records
		// not-found (preserves prior error semantics/messages).
		return nil, err
	}
	return execution.RunToTestRunRecord(run), nil
}

// runtimePersistenceStore backs workflow runtime persistence activities
// (execution.RunPersistenceActivities) — SP-E Task 3: retyped from
// *postgres.TestRunRepo/models.TestRunRecord to *postgres.RunRepo/models.Run,
// mirroring the cutover. Reads are by id because workflows know the run id,
// then writes go through the typed repo using the tenant carried in the
// record.
type runtimePersistenceStore struct {
	r    byIDReader
	runs *postgres.RunRepo
}

var _ execution.RunPersistenceStore = runtimePersistenceStore{}

func (s runtimePersistenceStore) RunRecord(ctx context.Context, runID string) (*modelspb.Run, error) {
	return s.r.run(ctx, runID)
}

func (s runtimePersistenceStore) SaveRunRecord(ctx context.Context, run *modelspb.Run) error {
	return s.runs.Update(ctx, run)
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

	Suites and suite runs were removed with the old test-orchestration backend
	(the suite/suite_run/suite_wizard services), so the dashboard's "Upcoming
	suites" tile is legitimately empty going forward — ListScheduledSuites
	reports no rows rather than reading a (now absent) suite repo.
*/

type dashboardRuns struct {
	runs *postgres.TestRunRepo
}

var _ adapters.DashboardRunsReader = dashboardRuns{}

func (d dashboardRuns) ListTenantRuns(ctx context.Context, tenantID string) ([]*modelspb.TestRunRecord, error) {
	out, _, err := d.runs.List(ctx, &apipb.ListTestRunsRequest{TenantId: tenantID}, "")
	return out, err
}

func (d dashboardRuns) ListScheduledSuites(context.Context, string) ([]adapters.ScheduledSuite, error) {
	return nil, nil
}
