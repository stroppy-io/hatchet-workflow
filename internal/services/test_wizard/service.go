package test_wizard

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	test_wizard implements cloud.v1.api.TestWizardService: the TEST wizard (the
	suite wizard is a separate service).

	The whole test form is ONE conditional schemapb schema held in
	TestWizardDraftRecord.form. The lifecycle is Start -> repeated Patch -> Finish.
	All branching/validation, the generated topology and readiness live in the
	schema + the WizardEngine; this service owns the wiring, tenant scoping,
	persistence and the gRPC contract. On Finish the engine bakes the ready draft
	into a domain.TestRun, which can additionally be launched and/or saved as a
	reusable test preset.

	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or schema logic of its own: every side effect goes
	through one of these. Persistence methods return derrors.ErrNotFound /
	derrors.ErrConflict so the handler can translate them via utils.MapErr.
*/

// DraftRepo persists TestWizardDraftRecord rows. Every row is tenant-scoped via
// its common.Entity; the repo MUST scope all reads/writes by tenant_id so a draft
// can never be read or mutated across tenants. Get/Update/Delete return
// derrors.ErrNotFound when the (tenant, id) pair is unknown.
type DraftRepo interface {
	Create(ctx context.Context, draft *models.TestWizardDraftRecord) error
	Get(ctx context.Context, tenantID, id string) (*models.TestWizardDraftRecord, error)
	List(ctx context.Context, tenantID string, filter *common.EntityFilter, sort *common.EntitySort, page *common.Page) (drafts []*models.TestWizardDraftRecord, nextPageToken string, err error)
	Update(ctx context.Context, draft *models.TestWizardDraftRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

// PresetReader loads an existing test preset to seed a wizard from. Get returns
// derrors.ErrNotFound when the (tenant, id) pair is unknown.
type PresetReader interface {
	Get(ctx context.Context, tenantID, presetID string) (*models.TestPresetRecord, error)
}

// WizardEngine is the schema brain of the wizard. It is provider-agnostic and
// stateless; the service feeds it forms and persists what it returns.
//
//   - InitialForm builds the starting conditional schema+values for a fresh
//     wizard, optionally seeded from an existing test preset (nil seed => blank
//     form).
//   - Compute validates a submitted form (honoring `when` gates), regenerates the
//     topology from the current values and recomputes readiness, returning the
//     (possibly re-emitted) form, the generated topology and the field errors. It
//     is pure: same form in => same result out, so it is safe to run inside a
//     retried transaction.
//   - Bake turns a ready draft into the materialized domain.TestRun (preset params
//   - provider settings + generated topology). It MUST reject a draft that is
//     not ready.
type WizardEngine interface {
	InitialForm(ctx context.Context, tenantID, name string, seed *models.TestPresetRecord) (*schemapb.Filled, error)
	Compute(ctx context.Context, tenantID string, form *schemapb.Filled) (newForm *schemapb.Filled, topo *topology.Topology, errs []*schemapb.FieldError, ready bool, err error)
	Bake(ctx context.Context, draft *models.TestWizardDraftRecord) (*domain.TestRun, error)
}

// TestRunStarter persists and launches a baked run via the TestRun API
// (TestWorkflow). It participates in the ambient ctx transaction so the
// persisted TestRunRecord commits atomically with the draft delete. trigger is
// MANUAL for a wizard-started run; inTenantRating/inGlobalRating carry the
// resolved rating membership.
type TestRunStarter interface {
	Start(ctx context.Context, tenantID string, run *domain.TestRun, trigger common.Trigger, inTenantRating, inGlobalRating bool) (*models.TestRunRecord, error)
}

// PresetSaver persists a baked run's db+workload as a reusable test preset. It
// participates in the ambient ctx transaction.
type PresetSaver interface {
	Save(ctx context.Context, tenantID, authorID, name string, run *domain.TestRun) (*models.TestPresetRecord, error)
}

// TestWizardDeps bundles every dependency for the constructor.
type TestWizardDeps struct {
	Authn   utils.Authn
	Drafts  DraftRepo
	Presets PresetReader
	Engine  WizardEngine
	Runs    TestRunStarter
	Saver   PresetSaver
	Tx      tx.Trm
}

type TestWizardService struct {
	*api.UnimplementedTestWizardServiceServer
	tx.Trm
	d TestWizardDeps
}

var _ api.TestWizardServiceServer = (*TestWizardService)(nil)

func NewTestWizardService(deps TestWizardDeps) *TestWizardService {
	return &TestWizardService{d: deps}
}

/*
	===== helpers =====
*/

func (s *TestWizardService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *TestWizardService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *TestWizardService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *TestWizardService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// loadDraft fetches a draft scoped to the tenant, translating not-found into a
// gRPC NotFound. It is the single ownership/tenant gate every per-draft RPC runs
// through, so a draft can never be touched from a foreign tenant.
func (s *TestWizardService) loadDraft(ctx context.Context, tenantID, draftID string) (*models.TestWizardDraftRecord, error) {
	d, err := s.d.Drafts.Get(ctx, tenantID, draftID)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return d, nil
}

// touch stamps updated_at on the draft, allocating the Timings chain if needed.
func (s *TestWizardService) touch(draft *models.TestWizardDraftRecord) {
	if draft.Entity == nil {
		return
	}
	if draft.Entity.Timings == nil {
		draft.Entity.Timings = &common.Timings{}
	}
	draft.Entity.Timings.UpdatedAt = s.now()
}

/*
	===== handlers =====
*/

// StartTestWizard opens a fresh draft. The form is built by the engine (blank, or
// seeded from test_preset_id when given), then immediately computed so the first
// draft the client sees already carries its topology/errors/readiness. The draft
// is tenant-scoped and the caller is stamped as author.
func (s *TestWizardService) StartTestWizard(ctx context.Context, req *api.StartTestWizardRequest) (*api.StartTestWizardResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	var seed *models.TestPresetRecord
	if req.GetTestPresetId() != "" {
		seed, err = s.d.Presets.Get(ctx, req.GetTenantId(), req.GetTestPresetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
	}

	form, err := s.d.Engine.InitialForm(ctx, req.GetTenantId(), req.GetName(), seed)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	form, topo, errs, ready, err := s.d.Engine.Compute(ctx, req.GetTenantId(), form)
	if err != nil {
		return nil, utils.MapErr(err)
	}

	draft := &models.TestWizardDraftRecord{
		Entity: &common.Entity{
			Id:       uuid.NewString(),
			TenantId: req.GetTenantId(),
			Name:     req.GetName(),
			AuthorId: c.GetAccountId(),
			Timings: &common.Timings{
				CreatedAt: s.now(),
				UpdatedAt: s.now(),
			},
		},
		Form:         form,
		Topology:     topo,
		Errors:       errs,
		Ready:        ready,
		TestPresetId: req.GetTestPresetId(),
	}
	if err := s.d.Drafts.Create(ctx, draft); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.StartTestWizardResponse{Draft: draft}, nil
}

// GetTestWizardDraft returns a single draft (tenant-scoped). Read-only.
func (s *TestWizardService) GetTestWizardDraft(ctx context.Context, req *api.GetTestWizardDraftRequest) (*api.GetTestWizardDraftResponse, error) {
	draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
	if err != nil {
		return nil, err
	}
	return &api.GetTestWizardDraftResponse{Draft: draft}, nil
}

// ListTestWizardDrafts lists drafts in the tenant, honoring the common Entity
// filter/sort/page (the UI filters by author and sorts by updated_at desc to
// offer "continue where you left off"). Read-only.
func (s *TestWizardService) ListTestWizardDrafts(ctx context.Context, req *api.ListTestWizardDraftsRequest) (*api.ListTestWizardDraftsResponse, error) {
	drafts, next, err := s.d.Drafts.List(ctx, req.GetTenantId(), req.GetFilter(), req.GetSort(), req.GetPage())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListTestWizardDraftsResponse{Drafts: drafts, NextPageToken: next}, nil
}

// PatchTestWizard applies the edited form: load the draft (tenant-scoped),
// recompute it through the engine (regenerating topology + readiness) and persist
// the new state. Idempotent — re-submitting the same form converges because
// Compute is pure, so the whole load/compute/store runs in one serializable
// transaction.
func (s *TestWizardService) PatchTestWizard(ctx context.Context, req *api.PatchTestWizardRequest) (*api.PatchTestWizardResponse, error) {
	draft, err := doTxRet(ctx, s, func(ctx context.Context) (*models.TestWizardDraftRecord, error) {
		draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
		if err != nil {
			return nil, err
		}
		form, topo, errs, ready, err := s.d.Engine.Compute(ctx, req.GetTenantId(), req.GetForm())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		draft.Form = form
		draft.Topology = topo
		draft.Errors = errs
		draft.Ready = ready
		s.touch(draft)
		if err := s.d.Drafts.Update(ctx, draft); err != nil {
			return nil, utils.MapErr(err)
		}
		return draft, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.PatchTestWizardResponse{Draft: draft}, nil
}

// DeleteTestWizardDraft drops a draft. Idempotent: deleting an absent draft is a
// no-op (not-found is swallowed).
func (s *TestWizardService) DeleteTestWizardDraft(ctx context.Context, req *api.DeleteTestWizardDraftRequest) (*api.DeleteTestWizardDraftResponse, error) {
	if err := derrors.IgnoreNotFound(s.d.Drafts.Delete(ctx, req.GetTenantId(), req.GetDraftId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteTestWizardDraftResponse{}, nil
}

// FinishTestWizard bakes a ready draft into a domain.TestRun. Rejected unless the
// draft is ready. Not idempotent — every call mints a new run. Optionally, in the
// same transaction, the baked run is started (persisted + TestWorkflow launched)
// and/or saved as a reusable test preset. The recompute + readiness check + bake
// + optional start/save all run in one serializable transaction so the readiness
// gate cannot race a concurrent patch and partial side effects cannot commit.
func (s *TestWizardService) FinishTestWizard(ctx context.Context, req *api.FinishTestWizardRequest) (*api.FinishTestWizardResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := doTxRet(ctx, s, func(ctx context.Context) (*api.FinishTestWizardResponse, error) {
		draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
		if err != nil {
			return nil, err
		}
		// Re-validate against the persisted form so a stale `ready` flag can never
		// let an invalid draft through.
		form, topo, errs, ready, err := s.d.Engine.Compute(ctx, req.GetTenantId(), draft.GetForm())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		draft.Form = form
		draft.Topology = topo
		draft.Errors = errs
		draft.Ready = ready
		if !ready {
			return nil, status.Error(codes.FailedPrecondition, "draft is not ready: resolve the validation errors and provider settings first")
		}

		run, err := s.d.Engine.Bake(ctx, draft)
		if err != nil {
			return nil, utils.MapErr(err)
		}

		out := &api.FinishTestWizardResponse{TestRun: run}

		if req.GetSaveAsPreset() {
			name := req.GetPresetName()
			if name == "" {
				name = draft.GetEntity().GetName()
			}
			preset, err := s.d.Saver.Save(ctx, req.GetTenantId(), c.GetAccountId(), name, run)
			if err != nil {
				return nil, utils.MapErr(err)
			}
			out.Preset = preset
		}

		if req.GetStart() {
			inTenant, inGlobal := resolveRating(req)
			rec, err := s.d.Runs.Start(ctx, req.GetTenantId(), run, common.Trigger_TRIGGER_MANUAL, inTenant, inGlobal)
			if err != nil {
				return nil, utils.MapErr(err)
			}
			out.Run = rec
		}

		// The draft has served its purpose once a run is minted; drop it so it does
		// not linger as a stale "continue" entry. Idempotent within the tx.
		if err := derrors.IgnoreNotFound(s.d.Drafts.Delete(ctx, req.GetTenantId(), req.GetDraftId())); err != nil {
			return nil, utils.MapErr(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// resolveRating applies the documented defaults for the started run's rating
// membership: unset in_tenant_rating -> true, unset in_global_rating -> false.
func resolveRating(req *api.FinishTestWizardRequest) (inTenant, inGlobal bool) {
	inTenant = true
	if req.InTenantRating != nil {
		inTenant = req.GetInTenantRating()
	}
	inGlobal = false
	if req.InGlobalRating != nil {
		inGlobal = req.GetInGlobalRating()
	}
	return inTenant, inGlobal
}
