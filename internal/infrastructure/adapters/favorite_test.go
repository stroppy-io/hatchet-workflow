package adapters

import (
	"context"
	"errors"
	"testing"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

func TestFavoriteTargetResolverResolvesRecipe(t *testing.T) {
	want := &common.Entity{Id: "recipe-1", TenantId: "tenant-1"}
	resolver := NewFavoriteTargetResolver(FavoriteTargetRepos{
		Recipes: EntityGetterFunc(func(ctx context.Context, tenantID, id string) (*common.Entity, error) {
			if tenantID != "tenant-1" || id != "recipe-1" {
				t.Fatalf("unexpected lookup: tenant=%s id=%s", tenantID, id)
			}
			return want, nil
		}),
	})

	got, err := resolver.Resolve(context.Background(), common.FavoriteKind_FAVORITE_KIND_RECIPE, "tenant-1", "recipe-1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != want {
		t.Fatalf("resolve: got %v, want %v", got, want)
	}
}

func TestFavoriteTargetResolverRecipeNotFound(t *testing.T) {
	resolver := NewFavoriteTargetResolver(FavoriteTargetRepos{
		Recipes: EntityGetterFunc(func(ctx context.Context, tenantID, id string) (*common.Entity, error) {
			return nil, derrors.NotFound("recipe", "recipe not found")
		}),
	})

	_, err := resolver.Resolve(context.Background(), common.FavoriteKind_FAVORITE_KIND_RECIPE, "tenant-1", "missing")
	if !errors.Is(err, derrors.ErrNotFound) {
		t.Fatalf("resolve: expected not-found error, got %v", err)
	}
}
