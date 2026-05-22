package ui

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui/uiconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// AuthoringActions is the dependency the AuthoringService is built on: the
// Wizard's stateless assembly RPC set (assemble, validate, render/probe preview,
// merge topology, materialize/check deployment, compile the DAG blueprint).
// internal/services/authoring implements it. Every RPC is unary and read-only
// (NO_SIDE_EFFECTS) — nothing here persists or schedules.
type AuthoringActions interface {
	AssembleFromPresets(ctx context.Context, req *uipb.AssembleFromPresetsRequest) (*uipb.TestPresetAssembly, error)
	AssembleTestPreset(ctx context.Context, req *uipb.AssembleTestPresetRequest) (*uipb.TestPresetAssembly, error)
	ValidateTestPreset(ctx context.Context, req *uipb.TestPresetRef) (*uipb.DiagnosticsReport, error)
	PreviewDatabaseRender(ctx context.Context, req *uipb.TestPresetRef) (*uipb.RenderPreview, error)
	ProbeWorkload(ctx context.Context, req *uipb.TestPresetRef) (*uipb.WorkloadProbe, error)
	PreviewWorkloadConfig(ctx context.Context, req *uipb.TestPresetRef) (*uipb.WorkloadConfigPreview, error)
	MergeTopology(ctx context.Context, req *uipb.MergeTopologyRequest) (*uipb.TopologyMerge, error)
	ValidateTopology(ctx context.Context, req *domain.Topology) (*uipb.DiagnosticsReport, error)
	MaterializeDeploymentIntent(ctx context.Context, req *uipb.MaterializeDeploymentIntentRequest) (*deployment.DeploymentIntent, error)
	ValidateDeploymentIntent(ctx context.Context, req *deployment.DeploymentIntent) (*uipb.DiagnosticsReport, error)
	CheckDeployment(ctx context.Context, req *uipb.TestPresetRef) (*uipb.DeploymentCheck, error)
	CompileTestPresetPreview(ctx context.Context, req *uipb.TestPresetRef) (*primitive.Dag, error)
}

// AuthoringService is the gRPC handler for cloud.v1.api.ui.AuthoringService. Pure
// transport: trace the call and delegate to svc. All methods are unary, so the
// simple-connect wrapper (uiconnect.NewAuthoringServiceHandler) accepts it.
type AuthoringService struct {
	uipb.UnimplementedAuthoringServiceServer
	*tracing.Entity
	svc AuthoringActions
}

var (
	_ uipb.AuthoringServiceServer       = (*AuthoringService)(nil)
	_ uiconnect.AuthoringServiceHandler = (*AuthoringService)(nil)
)

func NewAuthoringService(logger *xlog.Logger, svc AuthoringActions) *AuthoringService {
	return &AuthoringService{
		Entity: tracing.NewEntity(logger.AppendName("AuthoringService")),
		svc:    svc,
	}
}

func (s *AuthoringService) AssembleFromPresets(ctx context.Context, req *uipb.AssembleFromPresetsRequest) (*uipb.TestPresetAssembly, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "AssembleFromPresets",
		func(ctx context.Context, _ trace.Span) (*uipb.TestPresetAssembly, error) {
			return s.svc.AssembleFromPresets(ctx, req)
		})
}

func (s *AuthoringService) AssembleTestPreset(ctx context.Context, req *uipb.AssembleTestPresetRequest) (*uipb.TestPresetAssembly, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "AssembleTestPreset",
		func(ctx context.Context, _ trace.Span) (*uipb.TestPresetAssembly, error) {
			return s.svc.AssembleTestPreset(ctx, req)
		})
}

func (s *AuthoringService) ValidateTestPreset(ctx context.Context, req *uipb.TestPresetRef) (*uipb.DiagnosticsReport, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ValidateTestPreset",
		func(ctx context.Context, _ trace.Span) (*uipb.DiagnosticsReport, error) {
			return s.svc.ValidateTestPreset(ctx, req)
		})
}

func (s *AuthoringService) PreviewDatabaseRender(ctx context.Context, req *uipb.TestPresetRef) (*uipb.RenderPreview, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "PreviewDatabaseRender",
		func(ctx context.Context, _ trace.Span) (*uipb.RenderPreview, error) {
			return s.svc.PreviewDatabaseRender(ctx, req)
		})
}

func (s *AuthoringService) ProbeWorkload(ctx context.Context, req *uipb.TestPresetRef) (*uipb.WorkloadProbe, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ProbeWorkload",
		func(ctx context.Context, _ trace.Span) (*uipb.WorkloadProbe, error) {
			return s.svc.ProbeWorkload(ctx, req)
		})
}

func (s *AuthoringService) PreviewWorkloadConfig(ctx context.Context, req *uipb.TestPresetRef) (*uipb.WorkloadConfigPreview, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "PreviewWorkloadConfig",
		func(ctx context.Context, _ trace.Span) (*uipb.WorkloadConfigPreview, error) {
			return s.svc.PreviewWorkloadConfig(ctx, req)
		})
}

func (s *AuthoringService) MergeTopology(ctx context.Context, req *uipb.MergeTopologyRequest) (*uipb.TopologyMerge, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "MergeTopology",
		func(ctx context.Context, _ trace.Span) (*uipb.TopologyMerge, error) {
			return s.svc.MergeTopology(ctx, req)
		})
}

func (s *AuthoringService) ValidateTopology(ctx context.Context, req *domain.Topology) (*uipb.DiagnosticsReport, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ValidateTopology",
		func(ctx context.Context, _ trace.Span) (*uipb.DiagnosticsReport, error) {
			return s.svc.ValidateTopology(ctx, req)
		})
}

func (s *AuthoringService) MaterializeDeploymentIntent(ctx context.Context, req *uipb.MaterializeDeploymentIntentRequest) (*deployment.DeploymentIntent, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "MaterializeDeploymentIntent",
		func(ctx context.Context, _ trace.Span) (*deployment.DeploymentIntent, error) {
			return s.svc.MaterializeDeploymentIntent(ctx, req)
		})
}

func (s *AuthoringService) ValidateDeploymentIntent(ctx context.Context, req *deployment.DeploymentIntent) (*uipb.DiagnosticsReport, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ValidateDeploymentIntent",
		func(ctx context.Context, _ trace.Span) (*uipb.DiagnosticsReport, error) {
			return s.svc.ValidateDeploymentIntent(ctx, req)
		})
}

func (s *AuthoringService) CheckDeployment(ctx context.Context, req *uipb.TestPresetRef) (*uipb.DeploymentCheck, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CheckDeployment",
		func(ctx context.Context, _ trace.Span) (*uipb.DeploymentCheck, error) {
			return s.svc.CheckDeployment(ctx, req)
		})
}

func (s *AuthoringService) CompileTestPresetPreview(ctx context.Context, req *uipb.TestPresetRef) (*primitive.Dag, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CompileTestPresetPreview",
		func(ctx context.Context, _ trace.Span) (*primitive.Dag, error) {
			return s.svc.CompileTestPresetPreview(ctx, req)
		})
}
