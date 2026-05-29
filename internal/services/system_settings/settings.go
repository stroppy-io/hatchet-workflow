package system_settings

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// GetSystemSettings returns the singleton control-plane settings. admin_only —
// the RBAC interceptor enforces that upstream. On a fresh install where the row
// has never been written, the store reports derrors.ErrNotFound and we return the
// zero-valued defaults (all flags false, empty server_addr) rather than an error,
// so a never-configured control plane reads as its safe defaults.
func (s *SystemSettingsService) GetSystemSettings(ctx context.Context, _ *api.GetSystemSettingsRequest) (*api.GetSystemSettingsResponse, error) {
	settings, err := s.d.Settings.Get(ctx)
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.GetSystemSettingsResponse{Settings: &api.PlatformSettings{}}, nil
	}
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetSystemSettingsResponse{Settings: settings}, nil
}

// UpdateSystemSettings replaces the singleton wholesale. admin_only and
// IDEMPOTENT per the proto: re-sending the same payload yields the same row, so
// Set is an upsert of the single row and the response echoes the stored value.
func (s *SystemSettingsService) UpdateSystemSettings(ctx context.Context, req *api.UpdateSystemSettingsRequest) (*api.UpdateSystemSettingsResponse, error) {
	in := req.GetSettings()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}
	// Carry only the known fields onto a fresh message so unknown/unset bits never
	// leak through the wholesale replace.
	settings := &api.PlatformSettings{
		ServerAddr:                in.GetServerAddr(),
		AllowSelfRegistration:     in.GetAllowSelfRegistration(),
		AllowMemberTenantCreation: in.GetAllowMemberTenantCreation(),
	}
	out, err := doTxRet(ctx, s, func(ctx context.Context) (*api.PlatformSettings, error) {
		if err := s.d.Settings.Set(ctx, settings); err != nil {
			return nil, utils.MapErr(err)
		}
		return settings, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.UpdateSystemSettingsResponse{Settings: out}, nil
}
