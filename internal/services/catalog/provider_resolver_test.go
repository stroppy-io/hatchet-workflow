package catalog

import (
	"context"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
)

func TestCatalogProviderResolver_Unpinned_ResolvesLatest(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	ref1, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 1\n")})
	ref2, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 2\n")})
	e1 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 1)
	e1.SourceRef = ref1
	e2 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 2)
	e2.SourceRef = ref2
	repo.mustCreate(t, e1)
	repo.mustCreate(t, e2)

	resolver := &CatalogProviderResolver{Entries: repo, Bundles: store}
	files, version, err := resolver.ResolveProvider(context.Background(), "tenant-1", "yandex", 0)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if version != 2 {
		t.Fatalf("resolved version = %d, want 2 (latest)", version)
	}
	if string(files["manifest.yaml"]) != "name: yandex\nv: 2\n" {
		t.Fatalf("resolved wrong version's files: %q", files["manifest.yaml"])
	}
}

func TestCatalogProviderResolver_Pinned_ResolvesExactVersion(t *testing.T) {
	repo := newFakeEntryRepo()
	store := NewMemoryBundleStore()
	ref1, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 1\n")})
	ref2, _ := store.Write(context.Background(), "", map[string][]byte{"manifest.yaml": []byte("name: yandex\nv: 2\n")})
	e1 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 1)
	e1.SourceRef = ref1
	e2 := orgEntry(catalogpb.Kind_KIND_PROVIDER, "tenant-1", "yandex", 2)
	e2.SourceRef = ref2
	repo.mustCreate(t, e1)
	repo.mustCreate(t, e2)

	resolver := &CatalogProviderResolver{Entries: repo, Bundles: store}
	files, version, err := resolver.ResolveProvider(context.Background(), "tenant-1", "yandex", 1)
	if err != nil {
		t.Fatalf("ResolveProvider: %v", err)
	}
	if version != 1 || string(files["manifest.yaml"]) != "name: yandex\nv: 1\n" {
		t.Fatalf("expected pinned v1, got version=%d files=%q", version, files["manifest.yaml"])
	}
}

func TestCatalogProviderResolver_NotFound(t *testing.T) {
	resolver := &CatalogProviderResolver{Entries: newFakeEntryRepo(), Bundles: NewMemoryBundleStore()}
	_, _, err := resolver.ResolveProvider(context.Background(), "tenant-1", "nope", 0)
	if err == nil {
		t.Fatal("expected error for unknown provider slug")
	}
}
