package api

import (
	"context"

	"connectrpc.com/connect"
	apiv1 "github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1"
)

type settingsHandler struct {
	store *SettingsStore
}

func NewSettingsHandler(store *SettingsStore) *settingsHandler {
	return &settingsHandler{store: store}
}

func (h *settingsHandler) GetSettings(_ context.Context, _ *connect.Request[apiv1.GetSettingsRequest]) (*connect.Response[apiv1.GetSettingsResponse], error) {
	cfg, err := h.store.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&apiv1.GetSettingsResponse{Settings: cfg}), nil
}

func (h *settingsHandler) UpdateSettings(_ context.Context, req *connect.Request[apiv1.UpdateSettingsRequest]) (*connect.Response[apiv1.UpdateSettingsResponse], error) {
	if req.Msg.GetSettings() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}
	if err := h.store.Save(req.Msg.GetSettings()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&apiv1.UpdateSettingsResponse{Settings: req.Msg.GetSettings()}), nil
}
