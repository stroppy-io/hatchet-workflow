package catalog

import (
	"context"
	"fmt"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

// CatalogProviderResolver resolves a provider.use slug against the org
// catalog (SP-B §B5), backing internal/services/dsl.ProviderResolver via
// structural typing (that package deliberately does not import this one —
// see its own ProviderResolver doc comment). version == 0 resolves the
// latest org-catalog version of slug; a non-zero version resolves that exact
// CatalogEntry.version.
type CatalogProviderResolver struct {
	Entries CatalogEntryRepo
	Bundles BundleStore
}

// ResolveProvider looks up tenantID's org-catalog KIND_PROVIDER entry named
// slug (latest, or pinned to version when non-zero), then reads its bundle
// files from Bundles. A LINKED entry seeded by SeedOrgCatalog carries no
// source_ref of its own (see seed.go's doc) — its files live at the
// LEVEL_INSTANCE row it points at via source_entry_id, resolved here exactly
// like Service.ForkEntry does.
func (r *CatalogProviderResolver) ResolveProvider(ctx context.Context, tenantID, slug string, version uint32) (map[string][]byte, uint32, error) {
	var entry *catalogpb.CatalogEntry
	var err error
	if version == 0 {
		entry, err = r.Entries.GetLatestBySlug(ctx, catalogpb.Level_LEVEL_ORG, tenantID, catalogpb.Kind_KIND_PROVIDER, slug)
	} else {
		entry, err = r.Entries.GetBySlugVersion(ctx, catalogpb.Level_LEVEL_ORG, tenantID, catalogpb.Kind_KIND_PROVIDER, slug, version)
	}
	if err != nil {
		if derrors.IgnoreNotFound(err) == nil {
			return nil, 0, fmt.Errorf("provider %q not found in org catalog: %w", slug, err)
		}
		return nil, 0, err
	}

	ref := entry.GetSourceRef()
	if ref == "" {
		// LINKED, never forked — files live under the instance row it points at.
		src, err := r.Entries.Get(ctx, catalogpb.Level_LEVEL_INSTANCE, "", entry.GetSourceEntryId())
		if err != nil {
			return nil, 0, fmt.Errorf("provider %q: resolve linked source: %w", slug, err)
		}
		ref = src.GetSourceRef()
	}

	files, err := r.Bundles.Read(ctx, ref)
	if err != nil {
		return nil, 0, fmt.Errorf("provider %q: read bundle: %w", slug, err)
	}
	return files, entry.GetVersion(), nil
}
