package favorite

// service_test.go: unit tests for AddFavorite, RemoveFavorite, ListFavorites.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// -------- helpers --------

func newSvc(ctrl *gomock.Controller) (*FavoriteService, *utils.MockAuthn, *MockFavoriteRepo, *MockTargetResolver) {
	authn := utils.NewMockAuthn(ctrl)
	repo := NewMockFavoriteRepo(ctrl)
	targets := NewMockTargetResolver(ctrl)
	svc := NewFavoriteService(FavoriteDeps{
		Authn:     authn,
		Favorites: repo,
		Targets:   targets,
		Tx:        &utils.MockTrm{},
	})
	return svc, authn, repo, targets
}

const (
	accountID = "acc-1"
	tenantID  = "tenant-1"
	targetID  = "target-1"
)

var testKind = common.FavoriteKind_FAVORITE_KIND_DATABASE_PRESET

func callerClaims() *iampb.AccessClaims {
	return &iampb.AccessClaims{AccountId: accountID}
}

func existingRecord() *models.FavoriteRecord {
	return &models.FavoriteRecord{
		Entity: &common.Entity{
			Id:       "fav-1",
			TenantId: tenantID,
			AuthorId: accountID,
		},
		Kind:     testKind,
		TargetId: targetID,
	}
}

// -------- AddFavorite --------

func TestAddFavorite(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_NewRecord", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, targets := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		// No existing record
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(nil, derrors.ErrNotFound)
		// Target exists in the right tenant
		targets.EXPECT().Resolve(ctx, testKind, targetID).Return(&common.Entity{TenantId: tenantID}, nil)
		// Create succeeds
		repo.EXPECT().Create(ctx, accountID, gomock.Any()).Return(nil)

		resp, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err != nil {
			t.Fatalf("AddFavorite: %v", err)
		}
		if resp.Favorite == nil {
			t.Fatal("expected non-nil Favorite in response")
		}
	})

	t.Run("Idempotent_AlreadyFavorited", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		// Row already exists: return it immediately
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(existingRecord(), nil)

		resp, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err != nil {
			t.Fatalf("AddFavorite idempotent: %v", err)
		}
		if resp.Favorite == nil {
			t.Fatal("expected non-nil Favorite")
		}
	})

	t.Run("IdempotencyRace_ConflictOnCreate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, targets := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(nil, derrors.ErrNotFound)
		targets.EXPECT().Resolve(ctx, testKind, targetID).Return(&common.Entity{TenantId: tenantID}, nil)
		// Concurrent writer wins
		repo.EXPECT().Create(ctx, accountID, gomock.Any()).Return(derrors.ErrConflict)
		// Idempotency recovery get
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(existingRecord(), nil)

		resp, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err != nil {
			t.Fatalf("AddFavorite race: %v", err)
		}
		if resp.Favorite == nil {
			t.Fatal("expected non-nil Favorite after conflict recovery")
		}
	})

	t.Run("CallerError_Unauthenticated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("UnspecifiedKind_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)

		_, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED,
			TargetId: targetID,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("GetError_InternalPropagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(nil, errors.New("db boom"))

		_, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err == nil {
			t.Fatal("expected error propagated from Get")
		}
	})

	t.Run("TargetNotFound_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, targets := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(nil, derrors.ErrNotFound)
		targets.EXPECT().Resolve(ctx, testKind, targetID).Return(nil, derrors.ErrNotFound)

		_, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("TargetInDifferentTenant_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, targets := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(nil, derrors.ErrNotFound)
		// Target belongs to a different tenant
		targets.EXPECT().Resolve(ctx, testKind, targetID).Return(&common.Entity{TenantId: "other-tenant"}, nil)

		_, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound for cross-tenant, got %v", err)
		}
	})

	t.Run("CreateError_InternalPropagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, targets := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Get(ctx, accountID, testKind, targetID).Return(nil, derrors.ErrNotFound)
		targets.EXPECT().Resolve(ctx, testKind, targetID).Return(&common.Entity{TenantId: tenantID}, nil)
		repo.EXPECT().Create(ctx, accountID, gomock.Any()).Return(errors.New("insert failed"))

		_, err := svc.AddFavorite(ctx, &api.AddFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err == nil {
			t.Fatal("expected error from Create")
		}
	})
}

// -------- RemoveFavorite --------

func TestRemoveFavorite(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Delete(ctx, accountID, testKind, targetID).Return(nil)

		_, err := svc.RemoveFavorite(ctx, &api.RemoveFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err != nil {
			t.Fatalf("RemoveFavorite: %v", err)
		}
	})

	t.Run("Idempotent_AlreadyAbsent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		// Not-found is silently ignored (idempotent)
		repo.EXPECT().Delete(ctx, accountID, testKind, targetID).Return(derrors.ErrNotFound)

		_, err := svc.RemoveFavorite(ctx, &api.RemoveFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err != nil {
			t.Fatalf("RemoveFavorite idempotent: %v", err)
		}
	})

	t.Run("CallerError_Unauthenticated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.RemoveFavorite(ctx, &api.RemoveFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("UnspecifiedKind_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)

		_, err := svc.RemoveFavorite(ctx, &api.RemoveFavoriteRequest{
			TenantId: tenantID,
			Kind:     common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED,
			TargetId: targetID,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("DeleteError_InternalPropagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().Delete(ctx, accountID, testKind, targetID).Return(errors.New("db boom"))

		_, err := svc.RemoveFavorite(ctx, &api.RemoveFavoriteRequest{
			TenantId: tenantID,
			Kind:     testKind,
			TargetId: targetID,
		})
		if err == nil {
			t.Fatal("expected error from Delete")
		}
	})
}

// -------- ListFavorites --------

func TestListFavorites(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_AllKinds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		records := []*models.FavoriteRecord{existingRecord(), existingRecord()}
		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().List(ctx, accountID, tenantID,
			common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED, uint32(0), "").
			Return(records, "next-token", nil)

		resp, err := svc.ListFavorites(ctx, &api.ListFavoritesRequest{
			TenantId: tenantID,
			Kind:     common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED,
		})
		if err != nil {
			t.Fatalf("ListFavorites: %v", err)
		}
		if len(resp.Favorites) != 2 {
			t.Errorf("expected 2 records, got %d", len(resp.Favorites))
		}
		if resp.NextPageToken != "next-token" {
			t.Errorf("expected next-token, got %q", resp.NextPageToken)
		}
	})

	t.Run("Success_FilteredByKind", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().List(ctx, accountID, tenantID, testKind, uint32(10), "page-tok").
			Return([]*models.FavoriteRecord{existingRecord()}, "", nil)

		resp, err := svc.ListFavorites(ctx, &api.ListFavoritesRequest{
			TenantId: tenantID,
			Kind:     testKind,
			Page:     &common.Page{Size: 10, Token: "page-tok"},
		})
		if err != nil {
			t.Fatalf("ListFavorites filtered: %v", err)
		}
		if len(resp.Favorites) != 1 {
			t.Errorf("expected 1 record, got %d", len(resp.Favorites))
		}
	})

	t.Run("Success_EmptyList", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().List(ctx, accountID, tenantID,
			common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED, uint32(0), "").
			Return(nil, "", nil)

		resp, err := svc.ListFavorites(ctx, &api.ListFavoritesRequest{TenantId: tenantID})
		if err != nil {
			t.Fatalf("ListFavorites empty: %v", err)
		}
		if len(resp.Favorites) != 0 {
			t.Errorf("expected 0 records, got %d", len(resp.Favorites))
		}
	})

	t.Run("CallerError_Unauthenticated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, _, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no token"))

		_, err := svc.ListFavorites(ctx, &api.ListFavoritesRequest{TenantId: tenantID})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("ListError_InternalPropagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, authn, repo, _ := newSvc(ctrl)

		authn.EXPECT().Caller(ctx).Return(callerClaims(), nil)
		repo.EXPECT().List(ctx, accountID, tenantID,
			common.FavoriteKind_FAVORITE_KIND_UNSPECIFIED, uint32(0), "").
			Return(nil, "", errors.New("db boom"))

		_, err := svc.ListFavorites(ctx, &api.ListFavoritesRequest{TenantId: tenantID})
		if err == nil {
			t.Fatal("expected error from List")
		}
	})
}
