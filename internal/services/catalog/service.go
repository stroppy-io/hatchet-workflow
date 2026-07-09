package catalog

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Instance-level CRUD (admin_only, generic over Kind) =====
*/

// CreateInstanceEntry persists a new LEVEL_INSTANCE catalog entry. Platform-
// reserved (admin_only) — LEVEL_INSTANCE rows are visible to every tenant, so
// creating one is never RBAC-delegated.
func (s *Service) CreateInstanceEntry(ctx context.Context, req *catalogpb.CreateInstanceEntryRequest) (*catalogpb.CreateInstanceEntryResponse, error) {
	entry, err := s.createInstanceEntry(ctx, req.GetKind(), req.GetSlug(), req.GetName(), req.GetDescription(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CreateInstanceEntryResponse{Entry: entry}, nil
}

// UpdateInstanceEntry edits an existing LEVEL_INSTANCE entry's files in
// place (native origin only — every LEVEL_INSTANCE entry is native, there is
// nothing to fork at this level).
func (s *Service) UpdateInstanceEntry(ctx context.Context, req *catalogpb.UpdateInstanceEntryRequest) (*catalogpb.UpdateInstanceEntryResponse, error) {
	entry, err := s.updateInstanceEntry(ctx, req.GetId(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.UpdateInstanceEntryResponse{Entry: entry}, nil
}

// DeleteInstanceEntry removes a LEVEL_INSTANCE entry. Idempotent: deleting an
// absent or already-deleted id is a no-op, mirroring recipe.Service.
// DeleteRecipe.
func (s *Service) DeleteInstanceEntry(ctx context.Context, req *catalogpb.DeleteInstanceEntryRequest) (*catalogpb.DeleteInstanceEntryResponse, error) {
	if err := s.deleteInstanceEntry(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &catalogpb.DeleteInstanceEntryResponse{}, nil
}

// GetInstanceEntry reads one LEVEL_INSTANCE entry by id.
func (s *Service) GetInstanceEntry(ctx context.Context, req *catalogpb.GetInstanceEntryRequest) (*catalogpb.GetInstanceEntryResponse, error) {
	entry, err := s.getInstanceEntry(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &catalogpb.GetInstanceEntryResponse{Entry: entry}, nil
}

// GetInstanceEntryFiles reads a LEVEL_INSTANCE entry's stored bundle back.
func (s *Service) GetInstanceEntryFiles(ctx context.Context, req *catalogpb.GetInstanceEntryFilesRequest) (*catalogpb.GetInstanceEntryFilesResponse, error) {
	files, err := s.getEntryFiles(ctx, catalogpb.Level_LEVEL_INSTANCE, "", req.GetId(), catalogpb.Kind_KIND_UNSPECIFIED)
	if err != nil {
		return nil, err
	}
	return &catalogpb.GetInstanceEntryFilesResponse{Files: files}, nil
}

// ListInstanceEntries returns every LEVEL_INSTANCE entry of the requested
// kind.
func (s *Service) ListInstanceEntries(ctx context.Context, req *catalogpb.ListInstanceEntriesRequest) (*catalogpb.ListInstanceEntriesResponse, error) {
	entries, err := s.listEntries(ctx, catalogpb.Level_LEVEL_INSTANCE, "", req.GetKind())
	if err != nil {
		return nil, err
	}
	return &catalogpb.ListInstanceEntriesResponse{Entries: entries}, nil
}

/*
	===== Org-level CRUD, split per kind for all_of RBAC =====

	Every ...Provider/...Workflow pair below is a thin wrapper around one
	shared private helper (createOrgEntry, updateOrgEntry, ...): the PROTO
	surface (and therefore the RBAC annotation) is duplicated per kind so
	all_of can gate RESOURCE_PROVIDER separately from RESOURCE_WORKFLOW —
	the Go implementation behind it is not.
*/

func (s *Service) CreateOrgProvider(ctx context.Context, req *catalogpb.CreateOrgProviderRequest) (*catalogpb.CreateOrgProviderResponse, error) {
	entry, err := s.createOrgEntry(ctx, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetSlug(), req.GetName(), req.GetDescription(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CreateOrgProviderResponse{Entry: entry}, nil
}

func (s *Service) CreateOrgWorkflow(ctx context.Context, req *catalogpb.CreateOrgWorkflowRequest) (*catalogpb.CreateOrgWorkflowResponse, error) {
	entry, err := s.createOrgEntry(ctx, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetSlug(), req.GetName(), req.GetDescription(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CreateOrgWorkflowResponse{Entry: entry}, nil
}

func (s *Service) UpdateOrgProvider(ctx context.Context, req *catalogpb.UpdateOrgProviderRequest) (*catalogpb.UpdateOrgProviderResponse, error) {
	entry, err := s.updateOrgEntry(ctx, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetId(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.UpdateOrgProviderResponse{Entry: entry}, nil
}

func (s *Service) UpdateOrgWorkflow(ctx context.Context, req *catalogpb.UpdateOrgWorkflowRequest) (*catalogpb.UpdateOrgWorkflowResponse, error) {
	entry, err := s.updateOrgEntry(ctx, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetId(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.UpdateOrgWorkflowResponse{Entry: entry}, nil
}

func (s *Service) DeleteOrgProvider(ctx context.Context, req *catalogpb.DeleteOrgProviderRequest) (*catalogpb.DeleteOrgProviderResponse, error) {
	if err := s.deleteOrgEntry(ctx, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetId()); err != nil {
		return nil, err
	}
	return &catalogpb.DeleteOrgProviderResponse{}, nil
}

func (s *Service) DeleteOrgWorkflow(ctx context.Context, req *catalogpb.DeleteOrgWorkflowRequest) (*catalogpb.DeleteOrgWorkflowResponse, error) {
	if err := s.deleteOrgEntry(ctx, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetId()); err != nil {
		return nil, err
	}
	return &catalogpb.DeleteOrgWorkflowResponse{}, nil
}

func (s *Service) GetOrgProvider(ctx context.Context, req *catalogpb.GetOrgProviderRequest) (*catalogpb.GetOrgProviderResponse, error) {
	entry, err := s.getOrgEntry(ctx, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, err
	}
	return &catalogpb.GetOrgProviderResponse{Entry: entry}, nil
}

func (s *Service) GetOrgWorkflow(ctx context.Context, req *catalogpb.GetOrgWorkflowRequest) (*catalogpb.GetOrgWorkflowResponse, error) {
	entry, err := s.getOrgEntry(ctx, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, err
	}
	return &catalogpb.GetOrgWorkflowResponse{Entry: entry}, nil
}

// GetOrgProviderFiles reads a LEVEL_ORG KIND_PROVIDER entry's stored bundle back.
func (s *Service) GetOrgProviderFiles(ctx context.Context, req *catalogpb.GetOrgProviderFilesRequest) (*catalogpb.GetOrgProviderFilesResponse, error) {
	files, err := s.getEntryFiles(ctx, catalogpb.Level_LEVEL_ORG, req.GetTenantId(), req.GetId(), catalogpb.Kind_KIND_PROVIDER)
	if err != nil {
		return nil, err
	}
	return &catalogpb.GetOrgProviderFilesResponse{Files: files}, nil
}

// GetOrgWorkflowFiles reads a LEVEL_ORG KIND_WORKFLOW entry's stored bundle back.
func (s *Service) GetOrgWorkflowFiles(ctx context.Context, req *catalogpb.GetOrgWorkflowFilesRequest) (*catalogpb.GetOrgWorkflowFilesResponse, error) {
	files, err := s.getEntryFiles(ctx, catalogpb.Level_LEVEL_ORG, req.GetTenantId(), req.GetId(), catalogpb.Kind_KIND_WORKFLOW)
	if err != nil {
		return nil, err
	}
	return &catalogpb.GetOrgWorkflowFilesResponse{Files: files}, nil
}

func (s *Service) ListOrgProviders(ctx context.Context, req *catalogpb.ListOrgProvidersRequest) (*catalogpb.ListOrgProvidersResponse, error) {
	entries, err := s.listEntries(ctx, catalogpb.Level_LEVEL_ORG, req.GetTenantId(), catalogpb.Kind_KIND_PROVIDER)
	if err != nil {
		return nil, err
	}
	return &catalogpb.ListOrgProvidersResponse{Entries: entries}, nil
}

func (s *Service) ListOrgWorkflows(ctx context.Context, req *catalogpb.ListOrgWorkflowsRequest) (*catalogpb.ListOrgWorkflowsResponse, error) {
	entries, err := s.listEntries(ctx, catalogpb.Level_LEVEL_ORG, req.GetTenantId(), catalogpb.Kind_KIND_WORKFLOW)
	if err != nil {
		return nil, err
	}
	return &catalogpb.ListOrgWorkflowsResponse{Entries: entries}, nil
}

func (s *Service) LinkInstanceProvider(ctx context.Context, req *catalogpb.LinkInstanceProviderRequest) (*catalogpb.LinkInstanceProviderResponse, error) {
	entry, err := s.linkInstanceEntry(ctx, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetInstanceEntryId())
	if err != nil {
		return nil, err
	}
	return &catalogpb.LinkInstanceProviderResponse{Entry: entry}, nil
}

func (s *Service) LinkInstanceWorkflow(ctx context.Context, req *catalogpb.LinkInstanceWorkflowRequest) (*catalogpb.LinkInstanceWorkflowResponse, error) {
	entry, err := s.linkInstanceEntry(ctx, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetInstanceEntryId())
	if err != nil {
		return nil, err
	}
	return &catalogpb.LinkInstanceWorkflowResponse{Entry: entry}, nil
}

// CheckCatalogProvider re-runs the DSL check-mode compile pipeline over an
// UNSTORED bundle of files, without persisting anything — mirrors
// recipe.Service.CheckRecipe minus the repo lookup. Like every Check* RPC in
// this codebase, a problem in the bundle itself is NEVER an RPC error, only
// a diagnostic entry.
func (s *Service) CheckCatalogProvider(ctx context.Context, req *catalogpb.CheckCatalogProviderRequest) (*catalogpb.CheckCatalogProviderResponse, error) {
	diags, err := s.checkCatalog(ctx, catalogpb.Level_LEVEL_ORG, catalogpb.Kind_KIND_PROVIDER, req.GetTenantId(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CheckCatalogProviderResponse{Diagnostics: diags}, nil
}

func (s *Service) CheckCatalogWorkflow(ctx context.Context, req *catalogpb.CheckCatalogWorkflowRequest) (*catalogpb.CheckCatalogWorkflowResponse, error) {
	diags, err := s.checkCatalog(ctx, catalogpb.Level_LEVEL_ORG, catalogpb.Kind_KIND_WORKFLOW, req.GetTenantId(), req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CheckCatalogWorkflowResponse{Diagnostics: diags}, nil
}

// CheckInstanceProvider/CheckInstanceWorkflow are CheckCatalogProvider/
// CheckCatalogWorkflow's admin_only, tenant-less LEVEL_INSTANCE counterparts
// — see service.proto's file doc for why instance-scope Check cannot reuse
// the tenant-gated pair.
func (s *Service) CheckInstanceProvider(ctx context.Context, req *catalogpb.CheckInstanceProviderRequest) (*catalogpb.CheckInstanceProviderResponse, error) {
	diags, err := s.checkCatalog(ctx, catalogpb.Level_LEVEL_INSTANCE, catalogpb.Kind_KIND_PROVIDER, "", req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CheckInstanceProviderResponse{Diagnostics: diags}, nil
}

func (s *Service) CheckInstanceWorkflow(ctx context.Context, req *catalogpb.CheckInstanceWorkflowRequest) (*catalogpb.CheckInstanceWorkflowResponse, error) {
	diags, err := s.checkCatalog(ctx, catalogpb.Level_LEVEL_INSTANCE, catalogpb.Kind_KIND_WORKFLOW, "", req.GetFiles())
	if err != nil {
		return nil, err
	}
	return &catalogpb.CheckInstanceWorkflowResponse{Diagnostics: diags}, nil
}

/*
	===== shared private helpers =====

	One helper per verb, parameterized over level (and, for org RPCs, kind);
	every Org-/Instance-prefixed pair above and the generic instance handlers
	funnel through these, so the CRUD/versioning/summary logic exists exactly
	once regardless of how many proto RPCs front it.
*/

// createEntry is the shared implementation behind every Create* RPC
// (instance and org, every kind): validate the (level, tenantID) scope, run
// Check over the bundle, compute the next slug version, persist the bundle,
// derive the denormalized Summary, and persist the row as ORIGIN_NATIVE.
func (s *Service) createEntry(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind, slug, name, description string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	if err := requireLevel(level, tenantID); err != nil {
		return nil, err
	}
	diags, err := s.d.Check(ctx, kind, files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	version, err := s.nextVersion(ctx, level, tenantID, kind, slug)
	if err != nil {
		return nil, err
	}
	ref, err := s.d.Bundles.Write(ctx, "", files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	entry := &catalogpb.CatalogEntry{
		Entity: &common.Entity{
			Id:          uuid.NewString(),
			TenantId:    tenantID,
			Name:        name,
			Description: description,
			AuthorId:    c.GetAccountId(),
			Timings:     &common.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Level:     level,
		Kind:      kind,
		Slug:      slug,
		Version:   version,
		Origin:    catalogpb.Origin_ORIGIN_NATIVE,
		SourceRef: ref,
		Summary:   summaryFor(kind, files, diags),
	}
	if err := s.d.Entries.Create(ctx, entry); err != nil {
		return nil, utils.MapErr(err)
	}
	return entry, nil
}

func (s *Service) createOrgEntry(ctx context.Context, kind catalogpb.Kind, tenantID, slug, name, description string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	return s.createEntry(ctx, catalogpb.Level_LEVEL_ORG, tenantID, kind, slug, name, description, files)
}

func (s *Service) createInstanceEntry(ctx context.Context, kind catalogpb.Kind, slug, name, description string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	return s.createEntry(ctx, catalogpb.Level_LEVEL_INSTANCE, "", kind, slug, name, description, files)
}

// entryOfKind fetches (level, tenantID, id) and, when kindHint is not
// KIND_UNSPECIFIED, confirms the entry's own kind matches it. A mismatch
// surfaces as the same derrors.NotFound a genuinely absent id would: a
// caller holding only RESOURCE_PROVIDER's grant must not be able to
// discover that a given id belongs to a RESOURCE_WORKFLOW entry (or vice
// versa) by probing UpdateOrgProvider/GetOrgProvider/... with a workflow's
// id. KIND_UNSPECIFIED is passed by the generic instance-level handlers,
// which have no kind of their own to route by.
func (s *Service) entryOfKind(ctx context.Context, level catalogpb.Level, tenantID, id string, kindHint catalogpb.Kind) (*catalogpb.CatalogEntry, error) {
	entry, err := s.d.Entries.Get(ctx, level, tenantID, id)
	if err != nil {
		return nil, err
	}
	if kindHint != catalogpb.Kind_KIND_UNSPECIFIED && entry.GetKind() != kindHint {
		return nil, derrors.NotFound("catalog_entry", "catalog entry not found")
	}
	return entry, nil
}

// getEntry is the shared implementation behind every Get* RPC.
func (s *Service) getEntry(ctx context.Context, level catalogpb.Level, tenantID, id string, kindHint catalogpb.Kind) (*catalogpb.CatalogEntry, error) {
	if err := requireLevel(level, tenantID); err != nil {
		return nil, err
	}
	entry, err := s.entryOfKind(ctx, level, tenantID, id, kindHint)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return entry, nil
}

func (s *Service) getOrgEntry(ctx context.Context, kind catalogpb.Kind, tenantID, id string) (*catalogpb.CatalogEntry, error) {
	return s.getEntry(ctx, catalogpb.Level_LEVEL_ORG, tenantID, id, kind)
}

func (s *Service) getInstanceEntry(ctx context.Context, id string) (*catalogpb.CatalogEntry, error) {
	return s.getEntry(ctx, catalogpb.Level_LEVEL_INSTANCE, "", id, catalogpb.Kind_KIND_UNSPECIFIED)
}

// listEntries is the shared implementation behind every List* RPC.
func (s *Service) listEntries(ctx context.Context, level catalogpb.Level, tenantID string, kind catalogpb.Kind) ([]*catalogpb.CatalogEntry, error) {
	if err := requireLevel(level, tenantID); err != nil {
		return nil, err
	}
	entries, err := s.d.Entries.List(ctx, level, tenantID, kind)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return entries, nil
}

// updateEntry is the shared implementation behind every Update* RPC that
// edits a NATIVE or FORKED entry: rather than mutating the existing row in
// place, it creates a brand-new row at the same (level, tenantID, kind,
// slug) scope holding the edited files, stamped with the next free version
// — the existing row's bytes/summary are never touched, so a pinned
// "slug@version" resolving to it keeps resolving the same immutable bytes
// forever. Updating a LINKED row directly is refused — updateOrgEntry
// handles the fork-and-edit case itself instead of routing through here (see
// its doc); LEVEL_INSTANCE rows are always NATIVE, so this branch never
// fires for them.
func (s *Service) updateEntry(ctx context.Context, level catalogpb.Level, tenantID, id string, kindHint catalogpb.Kind, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	if err := requireLevel(level, tenantID); err != nil {
		return nil, err
	}
	current, err := s.entryOfKind(ctx, level, tenantID, id, kindHint)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if current.GetOrigin() == catalogpb.Origin_ORIGIN_LINKED {
		return nil, status.Error(codes.FailedPrecondition, "updating a linked catalog entry directly is not supported — fork it first")
	}
	return s.newVersion(ctx, current, current.GetOrigin(), files)
}

// newVersion is the shared implementation behind every edit path that must
// produce a new, immutable catalog version rather than mutate an existing
// row: updateEntry's NATIVE/FORKED path, and updateOrgEntry's LINKED path
// (which forces origin to FORKED). base supplies the (level, tenant, kind,
// slug) scope and the Entity.Name/Description/AuthorId/SourceEntryId
// carried forward onto the new row — base itself is never mutated or
// persisted again. The new row gets a fresh Entity.Id, the next free version
// for the scope, a new content-addressed bundle ref for files (so
// byte-identical edits naturally collapse to the same ref, and any
// non-identical edit gets its own — either way base's own ref is never
// touched), and a freshly derived Summary/Timings.
func (s *Service) newVersion(ctx context.Context, base *catalogpb.CatalogEntry, origin catalogpb.Origin, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	diags, err := s.d.Check(ctx, base.GetKind(), files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	ref, err := s.d.Bundles.Write(ctx, "", files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	level := base.GetLevel()
	tenantID := base.GetEntity().GetTenantId()
	nextVer, err := s.nextVersion(ctx, level, tenantID, base.GetKind(), base.GetSlug())
	if err != nil {
		return nil, err
	}
	now := s.now()
	entry := &catalogpb.CatalogEntry{
		Entity: &common.Entity{
			Id:          uuid.NewString(),
			TenantId:    tenantID,
			Name:        base.GetEntity().GetName(),
			Description: base.GetEntity().GetDescription(),
			AuthorId:    base.GetEntity().GetAuthorId(),
			Timings:     &common.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Level:         level,
		Kind:          base.GetKind(),
		Slug:          base.GetSlug(),
		Version:       nextVer,
		Origin:        origin,
		SourceEntryId: base.GetSourceEntryId(),
		SourceRef:     ref,
		Summary:       summaryFor(base.GetKind(), files, diags),
	}
	if err := s.d.Entries.Create(ctx, entry); err != nil {
		return nil, utils.MapErr(err)
	}
	return entry, nil
}

// updateOrgEntry is the shared implementation behind UpdateOrgProvider/
// UpdateOrgWorkflow: every edit — of a NATIVE, FORKED, or LINKED row —
// creates a brand-new immutable version at the next free (level, tenant,
// kind, slug) version rather than mutating an existing row (see newVersion).
// A LINKED row's edit stamps the new row ORIGIN_FORKED (the LINKED row
// itself, its source instance row, and every sibling org's LINKED row are
// left untouched); NATIVE/FORKED rows carry their own origin forward onto
// the new row. This deliberately does not compose through the standalone
// ForkEntry RPC (which also persists a version of its own, for the
// "customize before editing" UI action) — doing so would leave behind an
// extra unedited version nobody asked for; here the fork-and-edit happens in
// the single newVersion call below.
func (s *Service) updateOrgEntry(ctx context.Context, kind catalogpb.Kind, tenantID, id string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	if err := requireLevel(catalogpb.Level_LEVEL_ORG, tenantID); err != nil {
		return nil, err
	}
	current, err := s.entryOfKind(ctx, catalogpb.Level_LEVEL_ORG, tenantID, id, kind)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	origin := current.GetOrigin()
	if origin == catalogpb.Origin_ORIGIN_LINKED {
		origin = catalogpb.Origin_ORIGIN_FORKED
	}
	return s.newVersion(ctx, current, origin, files)
}

func (s *Service) updateInstanceEntry(ctx context.Context, id string, files map[string][]byte) (*catalogpb.CatalogEntry, error) {
	return s.updateEntry(ctx, catalogpb.Level_LEVEL_INSTANCE, "", id, catalogpb.Kind_KIND_UNSPECIFIED, files)
}

// deleteEntry is the shared implementation behind every Delete* RPC.
// Idempotent: deleting an absent id, or one that exists under a different
// kind than kindHint names, is a no-op — the same idempotency contract
// recipe.Service.DeleteRecipe documents.
func (s *Service) deleteEntry(ctx context.Context, level catalogpb.Level, tenantID, id string, kindHint catalogpb.Kind) error {
	if err := requireLevel(level, tenantID); err != nil {
		return err
	}
	if kindHint != catalogpb.Kind_KIND_UNSPECIFIED {
		if _, err := s.entryOfKind(ctx, level, tenantID, id, kindHint); err != nil {
			return utils.MapErr(derrors.IgnoreNotFound(err))
		}
	}
	return utils.MapErr(derrors.IgnoreNotFound(s.d.Entries.Delete(ctx, level, tenantID, id)))
}

func (s *Service) deleteOrgEntry(ctx context.Context, kind catalogpb.Kind, tenantID, id string) error {
	return s.deleteEntry(ctx, catalogpb.Level_LEVEL_ORG, tenantID, id, kind)
}

func (s *Service) deleteInstanceEntry(ctx context.Context, id string) error {
	return s.deleteEntry(ctx, catalogpb.Level_LEVEL_INSTANCE, "", id, catalogpb.Kind_KIND_UNSPECIFIED)
}

// linkInstanceEntry creates a new LEVEL_ORG row that references an existing
// LEVEL_INSTANCE entry without copying its content: Origin is LINKED,
// SourceEntryId points at the instance row, and SourceRef is shared with it
// (no new bundle write). The org row gets its own slug-scoped version
// counter, starting at 1 the same way a native Create would.
func (s *Service) linkInstanceEntry(ctx context.Context, kind catalogpb.Kind, tenantID, instanceEntryID string) (*catalogpb.CatalogEntry, error) {
	if err := requireLevel(catalogpb.Level_LEVEL_ORG, tenantID); err != nil {
		return nil, err
	}
	source, err := s.entryOfKind(ctx, catalogpb.Level_LEVEL_INSTANCE, "", instanceEntryID, kind)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	version, err := s.nextVersion(ctx, catalogpb.Level_LEVEL_ORG, tenantID, kind, source.GetSlug())
	if err != nil {
		return nil, err
	}
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	entry := &catalogpb.CatalogEntry{
		Entity: &common.Entity{
			Id:          uuid.NewString(),
			TenantId:    tenantID,
			Name:        source.GetEntity().GetName(),
			Description: source.GetEntity().GetDescription(),
			AuthorId:    c.GetAccountId(),
			Timings:     &common.Timings{CreatedAt: now, UpdatedAt: now},
		},
		Level:         catalogpb.Level_LEVEL_ORG,
		Kind:          kind,
		Slug:          source.GetSlug(),
		Version:       version,
		Origin:        catalogpb.Origin_ORIGIN_LINKED,
		SourceEntryId: source.GetEntity().GetId(),
		SourceRef:     source.GetSourceRef(),
		Summary:       source.GetSummary(),
	}
	if err := s.d.Entries.Create(ctx, entry); err != nil {
		return nil, utils.MapErr(err)
	}
	return entry, nil
}

// checkCatalog is the shared implementation behind CheckCatalogProvider/
// CheckCatalogWorkflow (level=LEVEL_ORG) and CheckInstanceProvider/
// CheckInstanceWorkflow (level=LEVEL_INSTANCE, tenantID=""): it never
// touches storage, only Check.
func (s *Service) checkCatalog(ctx context.Context, level catalogpb.Level, kind catalogpb.Kind, tenantID string, files map[string][]byte) ([]*dslpb.Diagnostic, error) {
	if err := requireLevel(level, tenantID); err != nil {
		return nil, err
	}
	diags, err := s.d.Check(ctx, kind, files)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return diags, nil
}

// getEntryFiles is the shared implementation behind every GetXFiles RPC: it
// resolves (level, tenantID, id) exactly like getEntry, then reads the
// entry's stored bundle from BundleStore. A LINKED row seeded with no
// source_ref of its own (see ForkEntry's doc in catalog.go) is resolved via
// its source_entry_id, mirroring ForkEntry's own fallback.
func (s *Service) getEntryFiles(ctx context.Context, level catalogpb.Level, tenantID, id string, kindHint catalogpb.Kind) (map[string][]byte, error) {
	if err := requireLevel(level, tenantID); err != nil {
		return nil, err
	}
	entry, err := s.entryOfKind(ctx, level, tenantID, id, kindHint)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	ref, err := s.resolveSourceRef(ctx, entry)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	if ref == "" {
		return nil, status.Error(codes.NotFound, "catalog entry has no stored bundle")
	}
	files, err := s.d.Bundles.Read(ctx, ref)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return files, nil
}

// resolveSourceRef returns entry's bundle ref, falling back to its source
// instance row's ref when entry itself carries none — the same fallback
// ForkEntry (catalog.go) uses for a LINKED row seeded by SeedOrgCatalog.
func (s *Service) resolveSourceRef(ctx context.Context, entry *catalogpb.CatalogEntry) (string, error) {
	if ref := entry.GetSourceRef(); ref != "" {
		return ref, nil
	}
	if entry.GetSourceEntryId() == "" {
		return "", nil
	}
	src, err := s.d.Entries.Get(ctx, catalogpb.Level_LEVEL_INSTANCE, "", entry.GetSourceEntryId())
	if err != nil {
		return "", err
	}
	return src.GetSourceRef(), nil
}

// summaryFor derives a new entry's denormalized Summary from its kind: a
// KIND_WORKFLOW bundle's Summary comes from its cluster.yaml (DeriveWorkflow
// Summary), a KIND_PROVIDER bundle's from its manifest.yaml
// (DeriveProviderSummary). An unparseable/missing manifest.yaml yields a
// zero-value provider Summary rather than failing the call — DecodeProvider
// Manifest's own diagnostics are not surfaced here (Check already ran over
// the same files and is what CatalogEntry.Summary.compiles reflects).
func summaryFor(kind catalogpb.Kind, files map[string][]byte, diags []*dslpb.Diagnostic) *catalogpb.CatalogEntry_Summary {
	if kind == catalogpb.Kind_KIND_WORKFLOW {
		return DeriveWorkflowSummary(files, diags)
	}
	manifest, _ := ast.DecodeProviderManifest("manifest.yaml", files["manifest.yaml"])
	summary := DeriveProviderSummary(manifest)
	summary.Compiles = len(diags) == 0
	return summary
}

// caller resolves the caller's verified access claims, mirroring
// recipe.Service.caller exactly: a resolution failure maps to
// Unauthenticated rather than silently stamping an empty author_id.
func (s *Service) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *Service) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}
