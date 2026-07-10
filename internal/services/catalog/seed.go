package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// SeedBuiltinCatalog seeds Deps.BuiltinProviders/BuiltinWorkflows as
// LEVEL_INSTANCE catalog entries, independent of any tenant existing yet
// (Task P1: a stand whose tenant predates SeedOrgCatalog's per-tenant-create
// wiring — internal/services/iam.Service's account-creation path — would
// otherwise never get a seeded catalog). Idempotent: safe to call on every
// boot. internal/app/run.go calls this once, right after constructing
// Service, in addition to (not instead of) SeedOrgCatalog's own call to the
// same underlying ensureBuiltinInstanceEntries — the two never race in
// practice (this runs once at boot, before request traffic starts) and
// ensureBuiltinInstanceEntries's GetLatestBySlug-then-Create check makes a
// second call from either path a no-op.
func (s *Service) SeedBuiltinCatalog(ctx context.Context) error {
	return s.ensureBuiltinInstanceEntries(ctx)
}

// SeedOrgCatalog links every LEVEL_INSTANCE entry into tenantID's org catalog
// as origin=LINKED rows (SP-B plan, live-link + fork-on-edit: no source_ref
// copy at seed time — a LINKED row reads its files through the instance row
// until its first edit forks it). Idempotent: a tenant that already has a
// LINKED row for a given instance entry is skipped, so calling this twice for
// the same tenant (or once per tenant against a shared instance catalog)
// never duplicates rows. Ensures builtin providers (e.g. docker) exist as
// instance entries first, so day-one tenants link against them too.
func (s *Service) SeedOrgCatalog(ctx context.Context, tenantID string) error {
	if err := s.ensureBuiltinInstanceEntries(ctx); err != nil {
		return err
	}
	for _, kind := range []catalogpb.Kind{catalogpb.Kind_KIND_PROVIDER, catalogpb.Kind_KIND_WORKFLOW} {
		instanceEntries, err := s.d.Entries.List(ctx, catalogpb.Level_LEVEL_INSTANCE, "", kind)
		if err != nil {
			return err
		}
		for _, ie := range instanceEntries {
			linked, err := s.d.Entries.ListBySource(ctx, ie.GetEntity().GetId())
			if err != nil {
				return err
			}
			if alreadyLinkedForTenant(linked, tenantID) {
				continue
			}
			now := s.now()
			org := &catalogpb.CatalogEntry{
				Entity: &common.Entity{
					Id:          uuid.NewString(),
					TenantId:    tenantID,
					Name:        ie.GetEntity().GetName(),
					Description: ie.GetEntity().GetDescription(),
					Timings:     &common.Timings{CreatedAt: now, UpdatedAt: now},
				},
				Level:         catalogpb.Level_LEVEL_ORG,
				Kind:          kind,
				Slug:          ie.GetSlug(),
				Version:       ie.GetVersion(),
				Origin:        catalogpb.Origin_ORIGIN_LINKED,
				SourceEntryId: ie.GetEntity().GetId(),
				Summary:       ie.GetSummary(),
			}
			if err := s.d.Entries.Create(ctx, org); err != nil && derrors.IgnoreConflict(err) != nil {
				return err
			}
		}
	}
	return nil
}

// alreadyLinkedForTenant reports whether linked (the rows already sourced
// from one instance entry, as returned by ListBySource) contains a row
// scoped to tenantID — SeedOrgCatalog's idempotency check.
func alreadyLinkedForTenant(linked []*catalogpb.CatalogEntry, tenantID string) bool {
	for _, l := range linked {
		if l.GetEntity().GetTenantId() == tenantID {
			return true
		}
	}
	return false
}

// ensureBuiltinInstanceEntries seeds Deps.BuiltinProviders (e.g. "docker",
// "yandex") as LEVEL_INSTANCE KIND_PROVIDER catalog entries and
// Deps.BuiltinWorkflows (e.g. "postgres-ha") as LEVEL_INSTANCE KIND_WORKFLOW
// catalog entries, if not already present. This is what lets the builtin
// providers/workflows participate in the catalog model as ordinary seeded
// LEVEL_INSTANCE rows rather than a hardcoded special case elsewhere in
// provider resolution (a later task unifies that lookup). Idempotent: a slug
// that already has a LEVEL_INSTANCE row of the matching kind is left alone.
// Called both from SeedOrgCatalog (per-tenant path, historical) and from
// internal/app's boot wiring directly (Task P1: a stand whose tenant
// predates SeedOrgCatalog's wiring must still get a seeded catalog).
func (s *Service) ensureBuiltinInstanceEntries(ctx context.Context) error {
	if err := s.ensureBuiltinKind(ctx, catalogpb.Kind_KIND_PROVIDER, s.d.BuiltinProviders); err != nil {
		return err
	}
	return s.ensureBuiltinKind(ctx, catalogpb.Kind_KIND_WORKFLOW, s.d.BuiltinWorkflows)
}

// ensureBuiltinKind is ensureBuiltinInstanceEntries's per-kind worker: it
// seeds one LEVEL_INSTANCE row per (slug, files) pair in sources that has no
// existing LEVEL_INSTANCE row of kind yet. Summary derivation reuses
// summaryFor (the same helper createEntry uses for user-authored entries),
// fed by an actual s.d.Check run so a builtin's Summary.compiles reflects
// reality instead of being hardcoded true.
func (s *Service) ensureBuiltinKind(ctx context.Context, kind catalogpb.Kind, sources map[string]map[string][]byte) error {
	for slug, files := range sources {
		_, err := s.d.Entries.GetLatestBySlug(ctx, catalogpb.Level_LEVEL_INSTANCE, "", kind, slug)
		if err == nil {
			continue
		}
		if !errors.Is(err, derrors.ErrNotFound) {
			return err
		}
		diags, err := s.d.Check(ctx, kind, files)
		if err != nil {
			return err
		}
		ref, err := s.d.Bundles.Write(ctx, identityFor(catalogpb.Level_LEVEL_INSTANCE, "", kind, slug), "", files)
		if err != nil {
			return err
		}
		now := s.now()
		entry := &catalogpb.CatalogEntry{
			Entity: &common.Entity{
				Id:      uuid.NewString(),
				Name:    slug,
				Timings: &common.Timings{CreatedAt: now, UpdatedAt: now},
			},
			Level:     catalogpb.Level_LEVEL_INSTANCE,
			Kind:      kind,
			Slug:      slug,
			Version:   1,
			Origin:    catalogpb.Origin_ORIGIN_NATIVE,
			SourceRef: ref,
			Summary:   summaryFor(kind, files, diags),
		}
		if err := s.d.Entries.Create(ctx, entry); err != nil && derrors.IgnoreConflict(err) != nil {
			return err
		}
	}
	return nil
}
