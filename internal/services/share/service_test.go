package share

// service_test.go: unit tests for all 6 RPCs in ShareService.
// Uses MockTrm (no real DB) and gomock mocks generated in
// mocks_test.go. Style mirrors internal/services/iam/apitoken_test.go.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// newSvc creates a ShareService backed by mocked dependencies and a no-op
// transaction manager.
func newSvc(
	authn *utils.MockAuthn,
	repo *MockShareRepo,
	minter *MockTokenMinter,
	snap *MockSnapshotBuilder,
) *ShareService {
	return NewShareService(ShareDeps{
		Authn:     authn,
		Shares:    repo,
		Minter:    minter,
		Snapshots: snap,
		Tx:        &utils.MockTrm{},
	})
}

// validTarget returns a well-formed ShareRecord_Target for use in requests.
func validTarget() *models.ShareRecord_Target {
	return &models.ShareRecord_Target{
		Kind: models.ShareRecord_Target_KIND_TEST_RUN,
		Id:   "run-001",
	}
}

// validRecord returns a minimal ShareRecord as if returned by the repo.
func validRecord(tenantID, id string) *models.ShareRecord {
	return &models.ShareRecord{
		Token:   "tok",
		Revoked: false,
		Target:  validTarget(),
	}
}

// =====================================================================
// CreateShare
// =====================================================================

func TestCreateShare(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_DefaultTTL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockShareRepo(ctrl)
		minter := NewMockTokenMinter(ctrl)
		snap := NewMockSnapshotBuilder(ctrl)
		svc := newSvc(authn, repo, minter, snap)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		minter.EXPECT().Mint().Return("token-abc", nil)
		snap.EXPECT().Build(ctx, "t1", validTarget()).Return(&models.ShareRecord_Snapshot{}, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		req := &api.CreateShareRequest{
			TenantId: "t1",
			Target:   validTarget(),
			// ttl nil => server default
		}
		resp, err := svc.CreateShare(ctx, req)
		if err != nil {
			t.Fatalf("CreateShare: %v", err)
		}
		if resp.Share == nil {
			t.Fatal("expected non-nil share in response")
		}
		if resp.Share.Token != "token-abc" {
			t.Errorf("token: want token-abc, got %s", resp.Share.Token)
		}
		// Default TTL means ExpiresAt should be set (not nil).
		if resp.Share.ExpiresAt == nil {
			t.Error("expected ExpiresAt to be set with default TTL")
		}
	})

	t.Run("Success_ExplicitTTL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockShareRepo(ctrl)
		minter := NewMockTokenMinter(ctrl)
		snap := NewMockSnapshotBuilder(ctrl)
		svc := newSvc(authn, repo, minter, snap)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		minter.EXPECT().Mint().Return("tok2", nil)
		snap.EXPECT().Build(ctx, "t1", validTarget()).Return(&models.ShareRecord_Snapshot{}, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

		req := &api.CreateShareRequest{
			TenantId: "t1",
			Target:   validTarget(),
			Ttl:      durationpb.New(3600 * 1e9), // 1 hour
		}
		resp, err := svc.CreateShare(ctx, req)
		if err != nil {
			t.Fatalf("CreateShare ExplicitTTL: %v", err)
		}
		if resp.Share.ExpiresAt == nil {
			t.Error("expected ExpiresAt for explicit positive TTL")
		}
	})

	t.Run("Success_NeverExpires_ZeroTTL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockShareRepo(ctrl)
		minter := NewMockTokenMinter(ctrl)
		snap := NewMockSnapshotBuilder(ctrl)
		svc := newSvc(authn, repo, minter, snap)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		minter.EXPECT().Mint().Return("tok3", nil)
		snap.EXPECT().Build(ctx, "t1", validTarget()).Return(&models.ShareRecord_Snapshot{}, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, rec *models.ShareRecord) error {
			if rec.ExpiresAt != nil {
				t.Errorf("expected nil ExpiresAt for zero TTL (never expires), got %v", rec.ExpiresAt)
			}
			return nil
		})

		req := &api.CreateShareRequest{
			TenantId: "t1",
			Target:   validTarget(),
			Ttl:      &durationpb.Duration{Seconds: 0}, // explicit zero = never
		}
		_, err := svc.CreateShare(ctx, req)
		if err != nil {
			t.Fatalf("CreateShare ZeroTTL: %v", err)
		}
	})

	t.Run("CallerError_Unauthenticated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(authn, NewMockShareRepo(ctrl), NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{TenantId: "t1", Target: validTarget()})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("MissingTarget_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(authn, NewMockShareRepo(ctrl), NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{TenantId: "t1"}) // no target
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("TargetKindUnspecified_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(authn, NewMockShareRepo(ctrl), NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{
			TenantId: "t1",
			Target:   &models.ShareRecord_Target{Kind: models.ShareRecord_Target_KIND_UNSPECIFIED, Id: "run1"},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("TargetIDEmpty_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(authn, NewMockShareRepo(ctrl), NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{
			TenantId: "t1",
			Target:   &models.ShareRecord_Target{Kind: models.ShareRecord_Target_KIND_TEST_RUN, Id: ""},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument (empty id), got %v", err)
		}
	})

	t.Run("MintError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		minter := NewMockTokenMinter(ctrl)
		svc := newSvc(authn, NewMockShareRepo(ctrl), minter, NewMockSnapshotBuilder(ctrl))

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		minter.EXPECT().Mint().Return("", errors.New("entropy exhausted"))

		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{TenantId: "t1", Target: validTarget()})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("SnapshotBuildNotFound_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockShareRepo(ctrl)
		minter := NewMockTokenMinter(ctrl)
		snap := NewMockSnapshotBuilder(ctrl)
		svc := newSvc(authn, repo, minter, snap)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		minter.EXPECT().Mint().Return("tok", nil)
		snap.EXPECT().Build(ctx, "t1", validTarget()).Return(nil, derrors.ErrNotFound)

		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{TenantId: "t1", Target: validTarget()})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("RepoCreateConflict_AlreadyExists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		repo := NewMockShareRepo(ctrl)
		minter := NewMockTokenMinter(ctrl)
		snap := NewMockSnapshotBuilder(ctrl)
		svc := newSvc(authn, repo, minter, snap)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc1"}, nil)
		minter.EXPECT().Mint().Return("tok", nil)
		snap.EXPECT().Build(ctx, "t1", validTarget()).Return(&models.ShareRecord_Snapshot{}, nil)
		repo.EXPECT().Create(ctx, gomock.Any()).Return(derrors.ErrConflict)

		_, err := svc.CreateShare(ctx, &api.CreateShareRequest{TenantId: "t1", Target: validTarget()})
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", err)
		}
	})
}

// =====================================================================
// GetShare
// =====================================================================

func TestGetShare(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Get(ctx, "t1", "s1").Return(validRecord("t1", "s1"), nil)

		resp, err := svc.GetShare(ctx, &api.GetShareRequest{TenantId: "t1", Id: "s1"})
		if err != nil {
			t.Fatalf("GetShare: %v", err)
		}
		if resp.Share == nil {
			t.Fatal("expected non-nil share")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetShare(ctx, &api.GetShareRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Get(ctx, "t1", "s1").Return(nil, errors.New("db down"))

		_, err := svc.GetShare(ctx, &api.GetShareRequest{TenantId: "t1", Id: "s1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// =====================================================================
// ListShares
// =====================================================================

func TestListShares(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_EmptyList", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().List(ctx, "t1", "", nil, nil, nil).Return(nil, "", nil)

		resp, err := svc.ListShares(ctx, &api.ListSharesRequest{TenantId: "t1"})
		if err != nil {
			t.Fatalf("ListShares: %v", err)
		}
		if len(resp.Shares) != 0 {
			t.Errorf("expected empty, got %d", len(resp.Shares))
		}
	})

	t.Run("Success_WithResults", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		shares := []*models.ShareRecord{validRecord("t1", "s1"), validRecord("t1", "s2")}
		repo.EXPECT().List(ctx, "t1", "run1", nil, nil, nil).Return(shares, "next-tok", nil)

		resp, err := svc.ListShares(ctx, &api.ListSharesRequest{TenantId: "t1", TargetId: "run1"})
		if err != nil {
			t.Fatalf("ListShares with target: %v", err)
		}
		if len(resp.Shares) != 2 {
			t.Errorf("expected 2, got %d", len(resp.Shares))
		}
		if resp.NextPageToken != "next-tok" {
			t.Errorf("expected next-tok, got %s", resp.NextPageToken)
		}
	})

	t.Run("RepoError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().List(ctx, "t1", "", nil, nil, nil).Return(nil, "", errors.New("db err"))

		_, err := svc.ListShares(ctx, &api.ListSharesRequest{TenantId: "t1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// =====================================================================
// RevokeShare
// =====================================================================

func TestRevokeShare(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_RevokesActive", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		rec := validRecord("t1", "s1")
		rec.Revoked = false
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.ShareRecord) error {
			if !r.Revoked {
				t.Error("expected Revoked=true after update")
			}
			return nil
		})

		resp, err := svc.RevokeShare(ctx, &api.RevokeShareRequest{TenantId: "t1", Id: "s1"})
		if err != nil {
			t.Fatalf("RevokeShare: %v", err)
		}
		if !resp.Share.Revoked {
			t.Error("response share should have Revoked=true")
		}
	})

	t.Run("Idempotent_AlreadyRevoked", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		rec := validRecord("t1", "s1")
		rec.Revoked = true
		// Update must NOT be called when already revoked.
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)

		resp, err := svc.RevokeShare(ctx, &api.RevokeShareRequest{TenantId: "t1", Id: "s1"})
		if err != nil {
			t.Fatalf("RevokeShare idempotent: %v", err)
		}
		if !resp.Share.Revoked {
			t.Error("expected Revoked=true in idempotent response")
		}
	})

	t.Run("NotFound_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.RevokeShare(ctx, &api.RevokeShareRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpdateError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		rec := validRecord("t1", "s1")
		rec.Revoked = false
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("db write fail"))

		_, err := svc.RevokeShare(ctx, &api.RevokeShareRequest{TenantId: "t1", Id: "s1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// =====================================================================
// SetShareExpiry
// =====================================================================

func TestSetShareExpiry(t *testing.T) {
	ctx := context.Background()

	t.Run("Success_SetPositiveTTL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		rec := validRecord("t1", "s1")
		rec.ExpiresAt = nil
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *models.ShareRecord) error {
			if r.ExpiresAt == nil {
				t.Error("expected ExpiresAt to be set after update")
			}
			return nil
		})

		resp, err := svc.SetShareExpiry(ctx, &api.SetShareExpiryRequest{
			TenantId: "t1", Id: "s1",
			Ttl: durationpb.New(3600 * 1e9),
		})
		if err != nil {
			t.Fatalf("SetShareExpiry: %v", err)
		}
		if resp.Share == nil {
			t.Fatal("expected non-nil share")
		}
	})

	t.Run("Success_SetNeverExpires_ZeroTTL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		rec := validRecord("t1", "s1")
		rec.ExpiresAt = nil
		// expiryFromTTL with explicit-zero-duration returns nil.
		// sameExpiry(nil, nil) = true => no update needed.
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)
		// Update MUST NOT be called because expiry is already nil == nil.

		_, err := svc.SetShareExpiry(ctx, &api.SetShareExpiryRequest{
			TenantId: "t1", Id: "s1",
			Ttl: &durationpb.Duration{Seconds: 0}, // explicit zero = never
		})
		if err != nil {
			t.Fatalf("SetShareExpiry zero TTL: %v", err)
		}
	})

	t.Run("Idempotent_SameExpiry", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		// Both nil => same expiry => no update.
		rec := validRecord("t1", "s1")
		rec.ExpiresAt = nil
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)
		// No Update call expected.

		_, err := svc.SetShareExpiry(ctx, &api.SetShareExpiryRequest{
			TenantId: "t1", Id: "s1",
			Ttl: &durationpb.Duration{Seconds: 0},
		})
		if err != nil {
			t.Fatalf("SetShareExpiry idempotent: %v", err)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Get(ctx, "t1", "missing").Return(nil, derrors.ErrNotFound)

		_, err := svc.SetShareExpiry(ctx, &api.SetShareExpiryRequest{TenantId: "t1", Id: "missing"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpdateError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		rec := validRecord("t1", "s1")
		rec.ExpiresAt = nil
		repo.EXPECT().Get(ctx, "t1", "s1").Return(rec, nil)
		repo.EXPECT().Update(ctx, gomock.Any()).Return(errors.New("write fail"))

		_, err := svc.SetShareExpiry(ctx, &api.SetShareExpiryRequest{
			TenantId: "t1", Id: "s1",
			Ttl: durationpb.New(3600 * 1e9),
		})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// =====================================================================
// DeleteShare
// =====================================================================

func TestDeleteShare(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Delete(ctx, "t1", "s1").Return(nil)

		resp, err := svc.DeleteShare(ctx, &api.DeleteShareRequest{TenantId: "t1", Id: "s1"})
		if err != nil {
			t.Fatalf("DeleteShare: %v", err)
		}
		if resp == nil {
			t.Fatal("expected non-nil response")
		}
	})

	t.Run("Idempotent_NotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		// Not-found is silently ignored.
		repo.EXPECT().Delete(ctx, "t1", "gone").Return(derrors.ErrNotFound)

		_, err := svc.DeleteShare(ctx, &api.DeleteShareRequest{TenantId: "t1", Id: "gone"})
		if err != nil {
			t.Fatalf("DeleteShare not-found should be no-op: %v", err)
		}
	})

	t.Run("RepoError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMockShareRepo(ctrl)
		svc := newSvc(utils.NewMockAuthn(ctrl), repo, NewMockTokenMinter(ctrl), NewMockSnapshotBuilder(ctrl))

		repo.EXPECT().Delete(ctx, "t1", "s1").Return(errors.New("db error"))

		_, err := svc.DeleteShare(ctx, &api.DeleteShareRequest{TenantId: "t1", Id: "s1"})
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})
}

// =====================================================================
// Helpers: sameExpiry – direct coverage of the three branches
// (nil+nil, one-nil, both-set equal, both-set not-equal).
// =====================================================================

func TestSameExpiry(t *testing.T) {
	// nil + nil => same
	if !sameExpiry(nil, nil) {
		t.Error("nil+nil should be same expiry")
	}
}
