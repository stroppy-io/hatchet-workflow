package stroppy

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel/trace"

	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/stroppybin"
	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
)

func (s *Service) ProbeStroppyConfig(ctx context.Context, req *stroppypb.ProbeStroppyConfigRequest) (*stroppypb.ProbeStroppyConfigResponse, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "ProbeStroppyConfig",
		func(ctx context.Context, _ trace.Span) (*stroppypb.ProbeStroppyConfigResponse, error) {
			files := make([]stroppybin.WorkloadFile, 0, len(req.GetFiles()))
			for _, f := range req.GetFiles() {
				files = append(files, stroppybin.WorkloadFile{Name: f.GetName(), Content: f.GetContent()})
			}
			result, err := s.runner.RunProbe(ctx, stroppybin.ProbeInput{
				Version:      req.GetStroppyVersion(),
				Script:       req.GetScript(),
				SQL:          req.GetSql(),
				DriverType:   req.GetDriverType(),
				PoolSize:     req.GetPoolSize(),
				ScaleFactor:  req.GetScaleFactor(),
				Env:          req.GetEnv(),
				Files:        files,
				IncludeHuman: req.GetIncludeHuman(),
			})
			if err != nil {
				return nil, err
			}
			resp := &stroppypb.ProbeStroppyConfigResponse{RawJson: string(result.StdoutJSON)}
			if req.GetIncludeHuman() {
				resp.Human = result.Human
			}
			var parsed struct {
				EnvDecls []struct {
					Name        string `json:"name"`
					Type        string `json:"type"`
					Default     string `json:"default"`
					Description string `json:"description"`
					Required    bool   `json:"required"`
				} `json:"env_declarations"`
				Steps []struct {
					Name        string `json:"name"`
					Kind        string `json:"kind"`
					Description string `json:"description"`
				} `json:"steps"`
				SQL []struct {
					Name string `json:"name"`
					Sql  string `json:"sql"`
				} `json:"sql_sections"`
				Drv []struct {
					DriverType string            `json:"driver_type"`
					Settings   map[string]string `json:"settings"`
				} `json:"driver_setups"`
			}
			if err := json.Unmarshal(result.StdoutJSON, &parsed); err == nil {
				for _, e := range parsed.EnvDecls {
					resp.EnvDeclarations = append(resp.EnvDeclarations, &stroppypb.ProbeEnvDecl{
						Name: e.Name, Type: e.Type, DefaultValue: e.Default,
						Description: e.Description, Required: e.Required,
					})
				}
				for _, st := range parsed.Steps {
					resp.Steps = append(resp.Steps, &stroppypb.ProbeStep{Name: st.Name, Kind: st.Kind, Description: st.Description})
				}
				for _, ss := range parsed.SQL {
					resp.SqlSections = append(resp.SqlSections, &stroppypb.ProbeSqlSection{Name: ss.Name, Sql: ss.Sql})
				}
				for _, d := range parsed.Drv {
					resp.DriverSetups = append(resp.DriverSetups, &stroppypb.ProbeDriverSetup{DriverType: d.DriverType, Settings: d.Settings})
				}
			}
			return resp, nil
		})
}
