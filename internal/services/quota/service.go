package quota

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

type RunReader interface {
	Get(ctx context.Context, tenantID, id string) (*models.TestRunRecord, error)
}

type Manager interface {
	ListQuotas(ctx context.Context, tenantID string, provider deploymentpb.Provider, policy api.QuotaRefreshPolicy) ([]*api.QuotaView, error)
	RefreshQuotas(ctx context.Context, tenantID string, provider deploymentpb.Provider) ([]*api.QuotaView, error)
	RunUsage(ctx context.Context, tenantID, runID string) ([]*api.QuotaReservationView, error)
}

type Deps struct {
	Authn   utils.Authn
	Tenants utils.TenantReader
	Runs    RunReader
	Quotas  Manager
}

type Service struct {
	*api.UnimplementedQuotaServiceServer
	d Deps
}

var _ api.QuotaServiceServer = (*Service)(nil)

func NewService(deps Deps) *Service {
	return &Service{UnimplementedQuotaServiceServer: &api.UnimplementedQuotaServiceServer{}, d: deps}
}

func (s *Service) ListQuotas(ctx context.Context, req *api.ListQuotasRequest) (*api.ListQuotasResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if s.d.Quotas == nil {
		return nil, status.Error(codes.Internal, "quota manager is not configured")
	}
	quotas, err := s.d.Quotas.ListQuotas(ctx, req.GetTenantId(), req.GetProvider(), req.GetRefreshPolicy())
	if err != nil {
		return nil, mapQuotaErr(err)
	}
	return &api.ListQuotasResponse{Quotas: quotas}, nil
}

func (s *Service) RefreshQuotas(ctx context.Context, req *api.RefreshQuotasRequest) (*api.RefreshQuotasResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if s.d.Quotas == nil {
		return nil, status.Error(codes.Internal, "quota manager is not configured")
	}
	quotas, err := s.d.Quotas.RefreshQuotas(ctx, req.GetTenantId(), req.GetProvider())
	if err != nil {
		return nil, mapQuotaErr(err)
	}
	return &api.RefreshQuotasResponse{Quotas: quotas}, nil
}

func (s *Service) GetRunQuotaUsage(ctx context.Context, req *api.GetRunQuotaUsageRequest) (*api.GetRunQuotaUsageResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.authorizeTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if s.d.Runs == nil {
		return nil, status.Error(codes.Internal, "run reader is not configured")
	}
	if _, err := s.d.Runs.Get(ctx, req.GetTenantId(), req.GetRunId()); err != nil {
		return nil, utils.MapErr(err)
	}
	if s.d.Quotas == nil {
		return nil, status.Error(codes.Internal, "quota manager is not configured")
	}
	reservations, err := s.d.Quotas.RunUsage(ctx, req.GetTenantId(), req.GetRunId())
	if err != nil {
		return nil, mapQuotaErr(err)
	}
	return &api.GetRunQuotaUsageResponse{Reservations: reservations}, nil
}

func (s *Service) authorizeTenant(ctx context.Context, tenantID string) error {
	if _, err := s.d.Authn.Caller(ctx); err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	if tenantID == "" {
		return status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if s.d.Tenants == nil {
		return status.Error(codes.Internal, "tenant reader is not configured")
	}
	if _, err := s.d.Tenants.Get(ctx, tenantID); err != nil {
		return utils.MapErr(err)
	}
	return nil
}

func mapQuotaErr(err error) error {
	var de *derrors.Error
	if errors.As(err, &de) {
		return utils.MapErr(err)
	}
	return status.Error(codes.Unavailable, err.Error())
}
