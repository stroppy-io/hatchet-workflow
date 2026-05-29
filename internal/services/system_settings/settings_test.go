package system_settings

// settings_test.go: unit tests for GetSystemSettings and UpdateSystemSettings RPCs.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// newSvc is a helper that creates a SystemSettingsService with the given mocks.
func newSvc(authn *utils.MockAuthn, settings *MockPlatformSettingsRepo) *SystemSettingsService {
	return NewSystemSettingsService(SystemSettingsDeps{
		Authn:    authn,
		Settings: settings,
		Tx:       &utils.MockTrm{},
	})
}

// -------- GetSystemSettings --------

func TestGetSystemSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	settings := NewMockPlatformSettingsRepo(ctrl)
	svc := newSvc(authn, settings)
	ctx := context.Background()

	t.Run("Success_WithStoredSettings", func(t *testing.T) {
		stored := &api.PlatformSettings{
			ServerAddr:                "https://example.com",
			AllowSelfRegistration:     true,
			AllowMemberTenantCreation: false,
		}
		settings.EXPECT().Get(ctx).Return(stored, nil)

		resp, err := svc.GetSystemSettings(ctx, &api.GetSystemSettingsRequest{})
		if err != nil {
			t.Fatalf("GetSystemSettings: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected non-nil settings in response")
		}
		if resp.Settings.ServerAddr != "https://example.com" {
			t.Errorf("expected server_addr=https://example.com, got %s", resp.Settings.ServerAddr)
		}
		if !resp.Settings.AllowSelfRegistration {
			t.Error("expected allow_self_registration=true")
		}
	})

	t.Run("Success_FreshInstall_ReturnsDefaults", func(t *testing.T) {
		// On a fresh install, Get returns ErrNotFound; handler should return defaults (zero values).
		settings.EXPECT().Get(ctx).Return(nil, derrors.ErrNotFound)

		resp, err := svc.GetSystemSettings(ctx, &api.GetSystemSettingsRequest{})
		if err != nil {
			t.Fatalf("GetSystemSettings fresh install: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected non-nil settings (defaults) in response")
		}
		if resp.Settings.ServerAddr != "" {
			t.Errorf("expected empty server_addr for defaults, got %s", resp.Settings.ServerAddr)
		}
		if resp.Settings.AllowSelfRegistration {
			t.Error("expected allow_self_registration=false for defaults")
		}
		if resp.Settings.AllowMemberTenantCreation {
			t.Error("expected allow_member_tenant_creation=false for defaults")
		}
	})

	t.Run("RepoError_ReturnsInternalError", func(t *testing.T) {
		settings.EXPECT().Get(ctx).Return(nil, errors.New("db connection failed"))

		_, err := svc.GetSystemSettings(ctx, &api.GetSystemSettingsRequest{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", status.Code(err))
		}
	})

	t.Run("DomainError_NotFound_ReturnsDefaults", func(t *testing.T) {
		// A richer domain not-found error (not just the sentinel) also yields defaults.
		richNotFound := derrors.NotFound("system_settings", "no settings row")
		settings.EXPECT().Get(ctx).Return(nil, richNotFound)

		resp, err := svc.GetSystemSettings(ctx, &api.GetSystemSettingsRequest{})
		if err != nil {
			t.Fatalf("expected no error for domain not-found, got %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected non-nil settings in response")
		}
	})
}

// -------- UpdateSystemSettings --------

func TestUpdateSystemSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	settings := NewMockPlatformSettingsRepo(ctrl)
	svc := newSvc(authn, settings)
	ctx := context.Background()

	t.Run("Success_FullSettings", func(t *testing.T) {
		in := &api.PlatformSettings{
			ServerAddr:                "https://ctrl.example.com",
			AllowSelfRegistration:     true,
			AllowMemberTenantCreation: true,
		}
		settings.EXPECT().Set(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, s *api.PlatformSettings) error {
			if s.ServerAddr != "https://ctrl.example.com" {
				t.Errorf("expected server_addr=https://ctrl.example.com, got %s", s.ServerAddr)
			}
			if !s.AllowSelfRegistration {
				t.Error("expected allow_self_registration=true")
			}
			if !s.AllowMemberTenantCreation {
				t.Error("expected allow_member_tenant_creation=true")
			}
			return nil
		})

		resp, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{Settings: in})
		if err != nil {
			t.Fatalf("UpdateSystemSettings: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected non-nil settings in response")
		}
		if resp.Settings.ServerAddr != "https://ctrl.example.com" {
			t.Errorf("expected server_addr=https://ctrl.example.com in response, got %s", resp.Settings.ServerAddr)
		}
	})

	t.Run("Success_Idempotent_SamePayloadTwice", func(t *testing.T) {
		in := &api.PlatformSettings{
			ServerAddr:            "https://ctrl.example.com",
			AllowSelfRegistration: false,
		}
		// Both calls should succeed (upsert semantics).
		settings.EXPECT().Set(ctx, gomock.Any()).Return(nil).Times(2)

		for i := 0; i < 2; i++ {
			resp, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{Settings: in})
			if err != nil {
				t.Fatalf("call %d: UpdateSystemSettings: %v", i+1, err)
			}
			if resp.Settings == nil {
				t.Fatalf("call %d: expected non-nil settings", i+1)
			}
		}
	})

	t.Run("NilSettings_InvalidArgument", func(t *testing.T) {
		_, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{Settings: nil})
		if err == nil {
			t.Fatal("expected error for nil settings")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", status.Code(err))
		}
	})

	t.Run("NilRequest_InvalidArgument", func(t *testing.T) {
		// GetSettings() on nil request returns nil PlatformSettings.
		_, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{})
		if err == nil {
			t.Fatal("expected error for missing settings field")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", status.Code(err))
		}
	})

	t.Run("RepoSetError_InternalError", func(t *testing.T) {
		in := &api.PlatformSettings{ServerAddr: "https://ctrl.example.com"}
		settings.EXPECT().Set(ctx, gomock.Any()).Return(errors.New("db write failed"))

		_, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{Settings: in})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal, got %v", status.Code(err))
		}
	})

	t.Run("RepoSetDomainConflict_AlreadyExists", func(t *testing.T) {
		in := &api.PlatformSettings{ServerAddr: "https://ctrl.example.com"}
		settings.EXPECT().Set(ctx, gomock.Any()).Return(derrors.Conflict("system_settings", "row already exists"))

		_, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{Settings: in})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists, got %v", status.Code(err))
		}
	})

	t.Run("OnlyKnownFieldsCopied", func(t *testing.T) {
		// Fields not in the known set should not propagate — implementation copies
		// only ServerAddr, AllowSelfRegistration, AllowMemberTenantCreation.
		in := &api.PlatformSettings{
			ServerAddr:                "https://ctrl.example.com",
			AllowSelfRegistration:     true,
			AllowMemberTenantCreation: false,
		}
		settings.EXPECT().Set(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, s *api.PlatformSettings) error {
			if s.ServerAddr != in.ServerAddr {
				t.Errorf("ServerAddr mismatch: %s", s.ServerAddr)
			}
			if s.AllowSelfRegistration != in.AllowSelfRegistration {
				t.Errorf("AllowSelfRegistration mismatch")
			}
			if s.AllowMemberTenantCreation != in.AllowMemberTenantCreation {
				t.Errorf("AllowMemberTenantCreation mismatch")
			}
			return nil
		})

		resp, err := svc.UpdateSystemSettings(ctx, &api.UpdateSystemSettingsRequest{Settings: in})
		if err != nil {
			t.Fatalf("UpdateSystemSettings: %v", err)
		}
		if resp.Settings.ServerAddr != in.ServerAddr {
			t.Errorf("response ServerAddr mismatch")
		}
	})
}
