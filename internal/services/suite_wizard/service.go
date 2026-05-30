package suite_wizard

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
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	suite_wizard implements cloud.v1.api.SuiteWizardService: the SUITE wizard.

	The whole suite form is ONE conditional schemapb schema held in
	SuiteWizardDraftRecord.form. The lifecycle is Start -> repeated Patch -> Finish.
	Matrix expansion (db x workload), workload<->db compatibility, per-topology
	provider settings and readiness all live in the schema + the WizardEngine; this
	service owns the wiring, tenant scoping, persistence and the gRPC contract.

	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or schema logic of its own: every side effect goes
	through one of these. Persistence methods return derrors.ErrNotFound /
	derrors.ErrConflict so the handler can translate them via utils.MapErr.
*/

// DraftRepo persists SuiteWizardDraftRecord rows. Every row is tenant-scoped via
// its common.Entity; the repo MUST scope all reads/writes by tenant_id so a draft
// can never be read or mutated across tenants. Get/Update/Delete return
// derrors.ErrNotFound when the (tenant, id) pair is unknown.
type DraftRepo interface {
	Create(ctx context.Context, draft *models.SuiteWizardDraftRecord) error
	Get(ctx context.Context, tenantID, id string) (*models.SuiteWizardDraftRecord, error)
	List(ctx context.Context, tenantID string, filter *common.EntityFilter, sort *common.EntitySort, page *common.Page) (drafts []*models.SuiteWizardDraftRecord, nextPageToken string, err error)
	Update(ctx context.Context, draft *models.SuiteWizardDraftRecord) error
	Delete(ctx context.Context, tenantID, id string) error
}

// SuiteReader loads an existing suite to seed a wizard from. Get returns
// derrors.ErrNotFound when the (tenant, id) pair is unknown.
type SuiteReader interface {
	Get(ctx context.Context, tenantID, suiteID string) (*domain.Suite, error)
}

// WizardEngine is the schema brain of the wizard. It is provider-agnostic and
// stateless; the service feeds it forms and persists what it returns.
//
//   - InitialForm builds the starting conditional schema+values for a fresh
//     wizard, optionally seeded from an existing suite (nil seed => blank form).
//   - Compute validates a submitted form, prunes the matrix to workload<->db
//     compatible cells, expands the preview, recomputes per-cell and overall
//     readiness and returns the (possibly re-emitted) form together with the
//     field errors. It is pure: same form in => same result out, so it is safe to
//     run inside a retried transaction.
//   - Bake turns a ready draft into the materialized domain.SuiteRun: every
//     compatible+ready cell becomes a fully baked TestRun (preset params +
//     provider settings + generated topology). It MUST reject a draft that is not
//     ready.
type WizardEngine interface {
	InitialForm(ctx context.Context, tenantID, name string, seed *domain.Suite) (*schemapb.Filled, error)
	// TODO(wizard-flow): when the concrete Compute is implemented, populate each
	// returned Cell.Errors from expand.Sanity (per-cell capacity/zone/quota
	// errors) so the preview surfaces per-cell validity alongside the overall form
	// errors.
	Compute(ctx context.Context, tenantID string, form *schemapb.Filled) (newForm *schemapb.Filled, preview []*models.SuiteWizardDraftRecord_Cell, errs []*schemapb.FieldError, ready bool, err error)
	Bake(ctx context.Context, draft *models.SuiteWizardDraftRecord) (*domain.SuiteRun, error)
}

// SuiteWizardDeps bundles every dependency for the constructor.
type SuiteWizardDeps struct {
	Authn  utils.Authn
	Drafts DraftRepo
	Suites SuiteReader
	Engine WizardEngine
	Tx     tx.Trm
}

type SuiteWizardService struct {
	*api.UnimplementedSuiteWizardServiceServer
	tx.Trm
	d SuiteWizardDeps
}

var _ api.SuiteWizardServiceServer = (*SuiteWizardService)(nil)

func NewSuiteWizardService(deps SuiteWizardDeps) *SuiteWizardService {
	return &SuiteWizardService{d: deps}
}

/*
	===== helpers =====
*/

func (s *SuiteWizardService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *SuiteWizardService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *SuiteWizardService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *SuiteWizardService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// loadDraft fetches a draft scoped to the tenant, translating not-found into a
// gRPC NotFound. It is the single ownership/tenant gate every per-draft RPC runs
// through, so a draft can never be touched from a foreign tenant.
func (s *SuiteWizardService) loadDraft(ctx context.Context, tenantID, draftID string) (*models.SuiteWizardDraftRecord, error) {
	d, err := s.d.Drafts.Get(ctx, tenantID, draftID)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return d, nil
}

/*
	===== handlers =====
*/

// StartSuiteWizard opens a fresh draft. The form is built by the engine (blank,
// or seeded from suite_id when given), then immediately computed so the first
// draft the client sees already carries its preview/errors/readiness. The draft
// is tenant-scoped and the caller is stamped as author.
func (s *SuiteWizardService) StartSuiteWizard(ctx context.Context, req *api.StartSuiteWizardRequest) (*api.StartSuiteWizardResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}

	var seed *domain.Suite
	if req.GetSuiteId() != "" {
		seed, err = s.d.Suites.Get(ctx, req.GetTenantId(), req.GetSuiteId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
	}

	form, err := s.d.Engine.InitialForm(ctx, req.GetTenantId(), req.GetName(), seed)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	form, preview, errs, ready, err := s.d.Engine.Compute(ctx, req.GetTenantId(), form)
	if err != nil {
		return nil, utils.MapErr(err)
	}

	draft := &models.SuiteWizardDraftRecord{
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
		Form:    form,
		Preview: preview,
		Errors:  errs,
		Ready:   ready,
	}
	if err := s.d.Drafts.Create(ctx, draft); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.StartSuiteWizardResponse{Draft: draft}, nil
}

// GetSuiteWizardDraft returns a single draft (tenant-scoped). Read-only.
func (s *SuiteWizardService) GetSuiteWizardDraft(ctx context.Context, req *api.GetSuiteWizardDraftRequest) (*api.GetSuiteWizardDraftResponse, error) {
	draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
	if err != nil {
		return nil, err
	}
	return &api.GetSuiteWizardDraftResponse{Draft: draft}, nil
}

// ListSuiteWizardDrafts lists drafts in the tenant, honoring the common Entity
// filter/sort/page. Read-only.
func (s *SuiteWizardService) ListSuiteWizardDrafts(ctx context.Context, req *api.ListSuiteWizardDraftsRequest) (*api.ListSuiteWizardDraftsResponse, error) {
	drafts, next, err := s.d.Drafts.List(ctx, req.GetTenantId(), req.GetFilter(), req.GetSort(), req.GetPage())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListSuiteWizardDraftsResponse{Drafts: drafts, NextPageToken: next}, nil
}

// PatchSuiteWizard applies the edited form: load the draft (tenant-scoped),
// recompute it through the engine and persist the new state. Idempotent —
// re-submitting the same form converges because Compute is pure, so the whole
// load/compute/store runs in one serializable transaction.
func (s *SuiteWizardService) PatchSuiteWizard(ctx context.Context, req *api.PatchSuiteWizardRequest) (*api.PatchSuiteWizardResponse, error) {
	draft, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteWizardDraftRecord, error) {
		draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
		if err != nil {
			return nil, err
		}
		form, preview, errs, ready, err := s.d.Engine.Compute(ctx, req.GetTenantId(), req.GetForm())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		draft.Form = form
		draft.Preview = preview
		draft.Errors = errs
		draft.Ready = ready
		if draft.Entity != nil {
			if draft.Entity.Timings == nil {
				draft.Entity.Timings = &common.Timings{}
			}
			draft.Entity.Timings.UpdatedAt = s.now()
		}
		if err := s.d.Drafts.Update(ctx, draft); err != nil {
			return nil, utils.MapErr(err)
		}
		return draft, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.PatchSuiteWizardResponse{Draft: draft}, nil
}

// DeleteSuiteWizardDraft drops a draft. Idempotent: deleting an absent draft is a
// no-op (not-found is swallowed).
func (s *SuiteWizardService) DeleteSuiteWizardDraft(ctx context.Context, req *api.DeleteSuiteWizardDraftRequest) (*api.DeleteSuiteWizardDraftResponse, error) {
	if err := derrors.IgnoreNotFound(s.d.Drafts.Delete(ctx, req.GetTenantId(), req.GetDraftId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteSuiteWizardDraftResponse{}, nil
}

// FinishSuiteWizard bakes a ready draft into a domain.SuiteRun. Rejected unless
// the draft is ready. Not idempotent — every call mints a new SuiteRun. The
// recompute + readiness check + bake run in one serializable transaction so the
// readiness gate cannot race a concurrent patch.
func (s *SuiteWizardService) FinishSuiteWizard(ctx context.Context, req *api.FinishSuiteWizardRequest) (*api.FinishSuiteWizardResponse, error) {
	run, err := doTxRet(ctx, s, func(ctx context.Context) (*domain.SuiteRun, error) {
		draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
		if err != nil {
			return nil, err
		}
		// Re-validate against the persisted form so a stale `ready` flag can never
		// let an invalid draft through.
		form, preview, errs, ready, err := s.d.Engine.Compute(ctx, req.GetTenantId(), draft.GetForm())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		draft.Form = form
		draft.Preview = preview
		draft.Errors = errs
		draft.Ready = ready
		if !ready {
			return nil, status.Error(codes.FailedPrecondition, "draft is not ready: resolve the validation errors and provider settings first")
		}
		run, err := s.d.Engine.Bake(ctx, draft)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		return run, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.FinishSuiteWizardResponse{SuiteRun: run}, nil
}
