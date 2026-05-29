package public_share

// get_shared_run_test.go: unit tests for GetSharedRun in service.go.

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// makeSnapshot returns a minimal non-nil ShareRecord_Snapshot.
func makeSnapshot() *models.ShareRecord_Snapshot {
	return &models.ShareRecord_Snapshot{
		CapturedAt: timestamppb.Now(),
	}
}

// liveRecord returns a ShareRecord that is not revoked and expires in the future.
func liveRecord(snap *models.ShareRecord_Snapshot) *models.ShareRecord {
	return &models.ShareRecord{
		Token:     "valid-token-abc",
		Revoked:   false,
		ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
		Snapshot:  snap,
	}
}

func TestGetSharedRun(t *testing.T) {
	ctx := context.Background()

	newSvc := func(ctrl *gomock.Controller, shares *MockShareRepo) *PublicShareService {
		return NewPublicShareService(PublicShareDeps{
			Shares: shares,
			Tx:     &utils.MockTrm{},
		})
	}

	t.Run("Success_WithSnapshot", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		snap := makeSnapshot()
		rec := liveRecord(snap)

		shares.EXPECT().GetByToken(ctx, "valid-token-abc").Return(rec, nil)

		resp, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "valid-token-abc"})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if resp.GetSnapshot() == nil {
			t.Fatal("expected snapshot in response")
		}
	})

	t.Run("EmptyToken_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("TokenNotFound_Gone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		shares.EXPECT().GetByToken(ctx, "missing-token-xyz").Return(nil, derrors.ErrNotFound)

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "missing-token-xyz"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound (gone), got %v", err)
		}
	})

	t.Run("RepoError_Internal", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		shares.EXPECT().GetByToken(ctx, "some-token-123").Return(nil, errors.New("db connection refused"))

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "some-token-123"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) == codes.NotFound {
			t.Errorf("non-NotFound repo error should not collapse to gone, got %v", err)
		}
	})

	t.Run("Revoked_Gone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		rec := &models.ShareRecord{
			Token:     "revoked-token-abc",
			Revoked:   true,
			ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
			Snapshot:  makeSnapshot(),
		}

		shares.EXPECT().GetByToken(ctx, "revoked-token-abc").Return(rec, nil)
		// isLive checks revoked first before calling clock.Now, so wall clock
		// is not consulted.

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "revoked-token-abc"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound (gone) for revoked share, got %v", err)
		}
	})

	t.Run("Expired_Gone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		rec := &models.ShareRecord{
			Token:     "expired-token-abc",
			Revoked:   false,
			ExpiresAt: timestamppb.New(time.Now().Add(-time.Hour)), // in the past
			Snapshot:  makeSnapshot(),
		}

		shares.EXPECT().GetByToken(ctx, "expired-token-abc").Return(rec, nil)

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "expired-token-abc"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound (gone) for expired share, got %v", err)
		}
	})

	t.Run("NilSnapshot_Gone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		rec := &models.ShareRecord{
			Token:     "no-snap-token-abc",
			Revoked:   false,
			ExpiresAt: timestamppb.New(time.Now().Add(time.Hour)),
			Snapshot:  nil, // background job hasn't run yet
		}

		shares.EXPECT().GetByToken(ctx, "no-snap-token-abc").Return(rec, nil)

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "no-snap-token-abc"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound (gone) for nil snapshot, got %v", err)
		}
	})

	t.Run("NilRecord_Gone", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		// Repo returns nil record without error (unusual but defensive).
		shares.EXPECT().GetByToken(ctx, "nil-record-token").Return(nil, nil)

		_, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "nil-record-token"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound (gone) for nil record, got %v", err)
		}
	})

	t.Run("NoExpiry_NeverExpires_Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shares := NewMockShareRepo(ctrl)
		svc := newSvc(ctrl, shares)

		snap := makeSnapshot()
		rec := &models.ShareRecord{
			Token:     "no-expiry-token-abc",
			Revoked:   false,
			ExpiresAt: nil, // no expiry
			Snapshot:  snap,
		}

		shares.EXPECT().GetByToken(ctx, "no-expiry-token-abc").Return(rec, nil)
		// isLive with nil ExpiresAt should not consult wall clock at all.

		resp, err := svc.GetSharedRun(ctx, &api.GetSharedRunRequest{Token: "no-expiry-token-abc"})
		if err != nil {
			t.Fatalf("expected success for no-expiry share, got: %v", err)
		}
		if resp.GetSnapshot() == nil {
			t.Fatal("expected snapshot in response")
		}
	})
}
