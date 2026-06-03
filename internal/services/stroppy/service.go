package stroppy

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// VersionSource lists suggested Stroppy release versions.
type VersionSource interface {
	List(ctx context.Context) ([]string, error)
}

// Deps bundles the StroppyService dependencies.
type Deps struct {
	Authn    utils.Authn
	Versions VersionSource
}

type Service struct {
	*api.UnimplementedStroppyServiceServer
	d Deps
}

var _ api.StroppyServiceServer = (*Service)(nil)

func NewService(deps Deps) *Service {
	return &Service{UnimplementedStroppyServiceServer: &api.UnimplementedStroppyServiceServer{}, d: deps}
}

// ListStroppyVersions returns server-approved suggested release versions. The
// auth interceptor enforces tenant membership + RESOURCE_WIZARD read permission;
// resolving the caller here keeps the handler consistently authenticated even if
// it is invoked outside the normal HTTP stack in tests.
func (s *Service) ListStroppyVersions(ctx context.Context, req *api.ListStroppyVersionsRequest) (*api.ListStroppyVersionsResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if _, err := s.d.Authn.Caller(ctx); err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	if s.d.Versions == nil {
		return nil, status.Error(codes.Internal, "stroppy version source is not configured")
	}
	versions, err := s.d.Versions.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &api.ListStroppyVersionsResponse{Versions: versions}, nil
}
