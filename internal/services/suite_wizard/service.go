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
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	suite_wizard implements cloud.v1.api.SuiteWizardService: the SUITE wizard.

	A suite draft is a structured SuiteWizardDraftRecord: one deployment provider
	plus a matrix of cells (database x workload), suite-level concurrency, schedule
	and rating defaults. The lifecycle is Start -> repeated Patch -> Finish. Patch
	mutates the draft's cell SPECS (and provider/concurrency/schedule/rating) and
	then recomputes the server-derived previews; the matrix expansion,
	workload<->db compatibility, per-cell render/infrastructure previews and
	readiness all live in the WizardEngine. This service owns the wiring (applying
	structured patches onto the persisted cell specs), tenant scoping, persistence
	and the gRPC contract.

	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or matrix logic of its own: every side effect goes
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

// WizardEngine is the matrix brain of the wizard. It is stateless; the service
// feeds it the draft's current spec state and persists what it returns.
//
//   - Initial builds the starting provider + cell list for a fresh wizard,
//     optionally seeded from an existing suite (nil seed => empty matrix). The
//     returned cells carry only their spec; previews/readiness are filled by the
//     subsequent Recompute.
//   - Recompute takes a draft whose cell SPECS (and provider/concurrency/etc.)
//     reflect the latest edits and resolves each cell: database/workload payloads,
//     topology spec, infrastructure plan and render preview, workload<->db
//     compatibility, per-cell and draft-level errors and readiness. It is pure:
//     same draft in => same result out, so it is safe to re-run inside a retried
//     transaction. It returns the fully-derived cell list plus the draft-level
//     errors and ready flag.
//   - Bake turns a ready draft into the persisted reusable suite and, when start
//     is requested, a prepared SuiteRun plus post-commit starter. Every
//     compatible+ready enabled cell becomes a fully baked TestRun (preset params
//   - provider settings + generated topology). It MUST reject a draft that is
//     not ready.
type WizardEngine interface {
	Initial(ctx context.Context, tenantID, name string, seed *domain.Suite) (provider deployment.Provider, cells []*models.SuiteWizardDraftRecord_Cell, err error)
	Recompute(ctx context.Context, tenantID string, draft *models.SuiteWizardDraftRecord) (cells []*models.SuiteWizardDraftRecord_Cell, errs []*schemapb.FieldError, ready bool, err error)
	// Bake persists the suite definition and, when start=true, prepares a
	// SuiteRun. suiteRun is nil when start=false. starter must be called only
	// after the surrounding transaction commits.
	Bake(ctx context.Context, draft *models.SuiteWizardDraftRecord, req *api.FinishSuiteWizardRequest) (suite *models.SuiteRecord, suiteRun *models.SuiteRunRecord, starter func(context.Context) error, err error)
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
	return &SuiteWizardService{UnimplementedSuiteWizardServiceServer: &api.UnimplementedSuiteWizardServiceServer{}, d: deps}
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

	provider, cells, err := s.d.Engine.Initial(ctx, req.GetTenantId(), req.GetName(), seed)
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
		Provider: provider,
		Cells:    cells,
		SuiteId:  req.GetSuiteId(),
	}

	// Compute the first preview so the draft the client sees already carries its
	// per-cell previews, errors and readiness.
	cells, errs, ready, err := s.d.Engine.Recompute(ctx, req.GetTenantId(), draft)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	draft.Cells = cells
	draft.Errors = errs
	draft.Ready = ready

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

// PatchSuiteWizard applies the structured edits: load the draft (tenant-scoped),
// merge the provider / concurrency / schedule / rating updates and the cell
// patches onto the persisted cell specs, recompute the previews through the
// engine and persist the new state. Idempotent — re-submitting the same edits
// converges because Recompute is pure, so the whole load/apply/compute/store runs
// in one serializable transaction.
func (s *SuiteWizardService) PatchSuiteWizard(ctx context.Context, req *api.PatchSuiteWizardRequest) (*api.PatchSuiteWizardResponse, error) {
	draft, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteWizardDraftRecord, error) {
		draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
		if err != nil {
			return nil, err
		}

		applyDraftEdits(draft, req)

		cells, errs, ready, err := s.d.Engine.Recompute(ctx, req.GetTenantId(), draft)
		if err != nil {
			return nil, utils.MapErr(err)
		}
		draft.Cells = cells
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

// applyDraftEdits merges the scalar suite-level updates and the cell patches from
// req onto draft's persisted cell SPECS. Server-derived cell fields (database,
// workload, previews, compatibility, readiness) are left untouched here; they are
// recomputed by the engine afterwards.
func applyDraftEdits(draft *models.SuiteWizardDraftRecord, req *api.PatchSuiteWizardRequest) {
	if req.GetProvider() != deployment.Provider_PROVIDER_UNSPECIFIED {
		draft.Provider = req.GetProvider()
	}
	if req.MaxParallel != nil {
		draft.MaxParallel = req.GetMaxParallel()
	}
	if req.GetSchedule() != nil {
		draft.Schedule = req.GetSchedule()
	}
	if req.DefaultInTenantRating != nil {
		v := req.GetDefaultInTenantRating()
		draft.DefaultInTenantRating = &v
	}
	if req.DefaultInGlobalRating != nil {
		v := req.GetDefaultInGlobalRating()
		draft.DefaultInGlobalRating = &v
	}

	if req.GetReplaceCells() {
		cells := make([]*models.SuiteWizardDraftRecord_Cell, 0, len(req.GetCells()))
		for _, p := range req.GetCells() {
			if p.GetRemove() {
				continue
			}
			cells = append(cells, &models.SuiteWizardDraftRecord_Cell{Spec: cellSpecFromPatch(p)})
		}
		draft.Cells = cells
		return
	}

	// Incremental merge by cell_id.
	for _, p := range req.GetCells() {
		mergeCellPatch(draft, p)
	}
}

// mergeCellPatch applies a single incremental cell patch onto draft.Cells,
// matching by cell_id: an empty cell_id creates a new cell, remove drops the
// matched cell, otherwise the matched cell's spec is updated in place.
func mergeCellPatch(draft *models.SuiteWizardDraftRecord, p *api.SuiteWizardCellPatch) {
	if p.GetCellId() == "" {
		if p.GetRemove() {
			return
		}
		draft.Cells = append(draft.Cells, &models.SuiteWizardDraftRecord_Cell{Spec: cellSpecFromPatch(p)})
		return
	}
	for i, cell := range draft.Cells {
		if cell.GetSpec().GetId() != p.GetCellId() {
			continue
		}
		if p.GetRemove() {
			draft.Cells = append(draft.Cells[:i], draft.Cells[i+1:]...)
			return
		}
		applyCellPatch(cell, p)
		return
	}
}

// cellSpecFromPatch builds a fresh SuiteCell spec from a create patch.
func cellSpecFromPatch(p *api.SuiteWizardCellPatch) *domain.SuiteCell {
	spec := &domain.SuiteCell{
		Id:      p.GetCellId(),
		Enabled: true,
	}
	if spec.Id == "" {
		spec.Id = uuid.NewString()
	}
	if p.Enabled != nil {
		spec.Enabled = p.GetEnabled()
	}
	if p.Name != nil {
		spec.Name = p.GetName()
	}
	setCellSource(spec, p)
	spec.MachineOverrides = p.GetMachineOverrides()
	spec.RenderOverrides = p.GetRenderOverrides()
	return spec
}

// applyCellPatch mutates an existing cell's spec from an incremental patch. Only
// fields present in the patch are overwritten.
func applyCellPatch(cell *models.SuiteWizardDraftRecord_Cell, p *api.SuiteWizardCellPatch) {
	if cell.Spec == nil {
		cell.Spec = &domain.SuiteCell{Id: p.GetCellId()}
	}
	if p.Enabled != nil {
		cell.Spec.Enabled = p.GetEnabled()
	}
	if p.Name != nil {
		cell.Spec.Name = p.GetName()
	}
	setCellSource(cell.Spec, p)
	if p.GetMachineOverrides() != nil {
		cell.Spec.MachineOverrides = p.GetMachineOverrides()
	}
	if p.GetRenderOverrides() != nil {
		cell.Spec.RenderOverrides = p.GetRenderOverrides()
	}
}

// setCellSource translates the cell-patch source oneof into the SuiteCell source
// oneof, writing it onto spec. When the patch sets no source the spec's existing
// source is left untouched.
func setCellSource(spec *domain.SuiteCell, p *api.SuiteWizardCellPatch) {
	switch src := p.GetSource().(type) {
	case *api.SuiteWizardCellPatch_PresetPair:
		spec.Source = &domain.SuiteCell_PresetPair_{PresetPair: src.PresetPair}
	case *api.SuiteWizardCellPatch_TestPresetId:
		spec.Source = &domain.SuiteCell_TestPresetId{TestPresetId: src.TestPresetId}
	case *api.SuiteWizardCellPatch_InlineTest:
		spec.Source = &domain.SuiteCell_InlineTest{InlineTest: src.InlineTest}
	}
}

// DeleteSuiteWizardDraft drops a draft. Idempotent: deleting an absent draft is a
// no-op (not-found is swallowed).
func (s *SuiteWizardService) DeleteSuiteWizardDraft(ctx context.Context, req *api.DeleteSuiteWizardDraftRequest) (*api.DeleteSuiteWizardDraftResponse, error) {
	if err := derrors.IgnoreNotFound(s.d.Drafts.Delete(ctx, req.GetTenantId(), req.GetDraftId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteSuiteWizardDraftResponse{}, nil
}

// finishResult carries the two artifacts Bake produces.
type finishResult struct {
	suite    *models.SuiteRecord
	suiteRun *models.SuiteRunRecord
	starter  func(context.Context) error
}

// FinishSuiteWizard bakes a ready draft into the persisted reusable suite and,
// when req.start is set, a prepared SuiteRun. SuiteWorkflow starts only after
// the transaction commits. Rejected unless the draft is ready. The recompute +
// readiness check + bake run in one serializable transaction so the readiness
// gate cannot race a concurrent patch.
func (s *SuiteWizardService) FinishSuiteWizard(ctx context.Context, req *api.FinishSuiteWizardRequest) (*api.FinishSuiteWizardResponse, error) {
	res, err := doTxRet(ctx, s, func(ctx context.Context) (finishResult, error) {
		draft, err := s.loadDraft(ctx, req.GetTenantId(), req.GetDraftId())
		if err != nil {
			return finishResult{}, err
		}
		// Re-validate against the persisted draft so a stale `ready` flag can never
		// let an invalid draft through.
		cells, errs, ready, err := s.d.Engine.Recompute(ctx, req.GetTenantId(), draft)
		if err != nil {
			return finishResult{}, utils.MapErr(err)
		}
		draft.Cells = cells
		draft.Errors = errs
		draft.Ready = ready
		if !ready {
			return finishResult{}, status.Error(codes.FailedPrecondition, "draft is not ready: resolve the validation errors and provider settings first")
		}
		suite, suiteRun, starter, err := s.d.Engine.Bake(ctx, draft, req)
		if err != nil {
			return finishResult{}, utils.MapErr(err)
		}
		return finishResult{suite: suite, suiteRun: suiteRun, starter: starter}, nil
	})
	if err != nil {
		return nil, err
	}
	if res.starter != nil {
		if err := res.starter(ctx); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &api.FinishSuiteWizardResponse{Suite: res.suite, SuiteRun: res.suiteRun}, nil
}
