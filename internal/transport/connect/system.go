package connect

import (
	"context"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
)

// ScheduleHandler implements systemconnect.ScheduleServiceHandler.
type ScheduleHandler struct{ svc *system.Service }

// NewScheduleHandler constructs a ScheduleHandler.
func NewScheduleHandler(svc *system.Service) *ScheduleHandler { return &ScheduleHandler{svc: svc} }

func (h *ScheduleHandler) CreateSchedule(ctx context.Context, req *connect.Request[systempb.CreateScheduleRequest]) (*connect.Response[systempb.Schedule], error) {
	result, err := h.svc.CreateSchedule(ctx, req.Msg.GetSchedule())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *ScheduleHandler) UpdateSchedule(ctx context.Context, req *connect.Request[systempb.UpdateScheduleRequest]) (*connect.Response[systempb.Schedule], error) {
	result, err := h.svc.UpdateSchedule(ctx, req.Msg.GetSchedule())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *ScheduleHandler) DeleteSchedule(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[systempb.Schedule], error) {
	existing, err := h.svc.GetSchedule(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteSchedule(ctx, req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(existing), nil
}

func (h *ScheduleHandler) GetSchedule(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[systempb.Schedule], error) {
	result, err := h.svc.GetSchedule(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

// ListSchedules ignores the tenant payload — schedules have no tenant_id column.
func (h *ScheduleHandler) ListSchedules(ctx context.Context, _ *connect.Request[iampb.TenantId]) (*connect.Response[systempb.Schedule_List], error) {
	items, err := h.svc.ListSchedules(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&systempb.Schedule_List{Schedules: items}), nil
}

func (h *ScheduleHandler) EnableSchedule(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[systempb.Schedule], error) {
	result, err := h.svc.EnableSchedule(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *ScheduleHandler) DisableSchedule(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[systempb.Schedule], error) {
	result, err := h.svc.DisableSchedule(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}

func (h *ScheduleHandler) TriggerNow(ctx context.Context, req *connect.Request[systempb.ScheduleId]) (*connect.Response[systempb.Schedule], error) {
	result, err := h.svc.TriggerNow(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(result), nil
}
