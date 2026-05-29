package system_settings

// service_helpers_test.go: tests for internal helpers in service.go
// (caller, now, doTx) that are not exercised by the current two RPCs but
// exist as scaffolding for future handlers.

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// TestCallerHelper tests the caller() private helper method.
func TestCallerHelper(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := NewSystemSettingsService(SystemSettingsDeps{
		Authn: authn,
		Tx:    &utils.MockTrm{},
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, nil)
		claims, err := svc.caller(ctx)
		if err != nil {
			t.Fatalf("caller: %v", err)
		}
		_ = claims
	})

	t.Run("AuthnError_Unauthenticated", func(t *testing.T) {
		authn.EXPECT().Caller(ctx).Return(nil, errors.New("token expired"))
		_, err := svc.caller(ctx)
		if err == nil {
			t.Fatal("expected error")
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", status.Code(err))
		}
	})
}

// TestNowHelper tests the now() private helper method.
func TestNowHelper(t *testing.T) {
	svc := NewSystemSettingsService(SystemSettingsDeps{
		Tx: &utils.MockTrm{},
	})

	ts := svc.now()
	if ts == nil {
		t.Fatal("expected non-nil timestamp")
	}
	// The returned timestamp should be close to current time.
	diff := time.Since(ts.AsTime())
	if diff < 0 || diff > 5*time.Second {
		t.Errorf("timestamp too far from now: diff=%v", diff)
	}
}

// TestDoTxHelper tests the doTx() private helper method.
func TestDoTxHelper(t *testing.T) {
	svc := NewSystemSettingsService(SystemSettingsDeps{
		Tx: &utils.MockTrm{},
	})
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		called := false
		err := svc.doTx(ctx, func(ctx context.Context) error {
			called = true
			return nil
		})
		if err != nil {
			t.Fatalf("doTx: %v", err)
		}
		if !called {
			t.Error("expected fn to be called")
		}
	})

	t.Run("PropagatesError", func(t *testing.T) {
		want := errors.New("tx fn error")
		err := svc.doTx(ctx, func(ctx context.Context) error {
			return want
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, want) {
			t.Errorf("expected wrapped error, got %v", err)
		}
	})
}
