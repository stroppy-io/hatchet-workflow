package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

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

// ensureBuiltinInstanceEntries seeds Deps.BuiltinProviders (e.g. "docker") as
// LEVEL_INSTANCE catalog entries if not already present. This is what lets
// the docker builtin provider participate in the catalog model as an
// ordinary seeded LEVEL_INSTANCE row rather than a hardcoded special case
// elsewhere in provider resolution (a later task unifies that lookup).
// Idempotent: a slug that already has a LEVEL_INSTANCE row is left alone.
func (s *Service) ensureBuiltinInstanceEntries(ctx context.Context) error {
	for slug, files := range s.d.BuiltinProviders {
		_, err := s.d.Entries.GetLatestBySlug(ctx, catalogpb.Level_LEVEL_INSTANCE, "", catalogpb.Kind_KIND_PROVIDER, slug)
		if err == nil {
			continue
		}
		if !errors.Is(err, derrors.ErrNotFound) {
			return err
		}
		ref, err := s.d.Bundles.Write(ctx, "", files)
		if err != nil {
			return err
		}
		manifest, _ := ast.DecodeProviderManifest("manifest.yaml", files["manifest.yaml"])
		now := s.now()
		entry := &catalogpb.CatalogEntry{
			Entity: &common.Entity{
				Id:      uuid.NewString(),
				Name:    slug,
				Timings: &common.Timings{CreatedAt: now, UpdatedAt: now},
			},
			Level:     catalogpb.Level_LEVEL_INSTANCE,
			Kind:      catalogpb.Kind_KIND_PROVIDER,
			Slug:      slug,
			Version:   1,
			Origin:    catalogpb.Origin_ORIGIN_NATIVE,
			SourceRef: ref,
			Summary:   DeriveProviderSummary(manifest),
		}
		if err := s.d.Entries.Create(ctx, entry); err != nil && derrors.IgnoreConflict(err) != nil {
			return err
		}
	}
	return nil
}
