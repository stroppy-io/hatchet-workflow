package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// PresetActions is the dependency the PresetService is built on: tenant-scoped
// reusable Preset{DATABASE|WORKLOAD|TEST} catalog. internal/services implements it.
type PresetActions interface {
	ListPresets(ctx context.Context, req *uipb.ListPresetRequest) (*models.Preset_List, error)
	CreatePreset(ctx context.Context, req *models.Preset) (*models.Preset, error)
	UpdatePreset(ctx context.Context, req *models.Preset) (*models.Preset, error)
	DeletePreset(ctx context.Context, req *uipb.DeletePresetRequest) (*emptypb.Empty, error)
	ClonePreset(ctx context.Context, req *uipb.ClonePresetRequest) (*models.Preset, error)
}

// PresetService is the gRPC handler for cloud.v1.api.ui.PresetService. Pure
// transport: trace the call and delegate to svc.
type PresetService struct {
	uipb.UnimplementedPresetServiceServer
	*tracing.Entity
	svc PresetActions
}

var _ uipb.PresetServiceServer = (*PresetService)(nil)

func NewPresetService(logger *xlog.Logger, svc PresetActions) *PresetService {
	return &PresetService{
		Entity: tracing.NewEntity(logger.AppendName("PresetService")),
		svc:    svc,
	}
}

func (s *PresetService) ListPresets(ctx context.Context, req *uipb.ListPresetRequest) (*models.Preset_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListPresets",
		func(ctx context.Context, _ trace.Span) (*models.Preset_List, error) {
			return s.svc.ListPresets(ctx, req)
		})
}

func (s *PresetService) CreatePreset(ctx context.Context, req *models.Preset) (*models.Preset, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreatePreset",
		func(ctx context.Context, _ trace.Span) (*models.Preset, error) {
			return s.svc.CreatePreset(ctx, req)
		})
}

func (s *PresetService) UpdatePreset(ctx context.Context, req *models.Preset) (*models.Preset, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdatePreset",
		func(ctx context.Context, _ trace.Span) (*models.Preset, error) {
			return s.svc.UpdatePreset(ctx, req)
		})
}

func (s *PresetService) DeletePreset(ctx context.Context, req *uipb.DeletePresetRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeletePreset",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.DeletePreset(ctx, req)
		})
}

func (s *PresetService) ClonePreset(ctx context.Context, req *uipb.ClonePresetRequest) (*models.Preset, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ClonePreset",
		func(ctx context.Context, _ trace.Span) (*models.Preset, error) {
			return s.svc.ClonePreset(ctx, req)
		})
}
