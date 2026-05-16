package stroppy

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

func (s *Service) PreviewStroppyConfig(ctx context.Context, req *stroppypb.PreviewStroppyConfigRequest) (*stroppypb.PreviewStroppyConfigResponse, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "PreviewStroppyConfig",
		func(ctx context.Context, _ trace.Span) (*stroppypb.PreviewStroppyConfigResponse, error) {
			if override := req.GetConfigOverrideJson(); override != "" {
				return &stroppypb.PreviewStroppyConfigResponse{StroppyConfigJson: override}, nil
			}
			files := make([]stroppybin.WorkloadFile, 0, len(req.GetFiles()))
			for _, f := range req.GetFiles() {
				files = append(files, stroppybin.WorkloadFile{Name: f.GetName(), Content: f.GetContent()})
			}
			data, err := stroppybin.RenderConfig(ctx, stroppybin.PreviewInput{
				Version: req.GetStroppyVersion(), Script: req.GetScript(), SQL: req.GetSql(),
				DriverType: req.GetDriverType(), PoolSize: req.GetPoolSize(), ScaleFactor: req.GetScaleFactor(),
				Env: req.GetEnv(), Files: files,
			})
			if err != nil {
				return nil, err
			}
			return &stroppypb.PreviewStroppyConfigResponse{StroppyConfigJson: string(data)}, nil
		})
}
