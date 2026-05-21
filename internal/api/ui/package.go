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

// PackageActions is the dependency the PackageService is built on: tenant-scoped
// custom package catalog. Upload is via presigned PUT (RequestPackageUpload
// returns the URL; bytes go over HTTP, not RPC). internal/services implements it.
type PackageActions interface {
	ListPackages(ctx context.Context, req *uipb.ListPackagesRequest) (*models.Package_List, error)
	RequestPackageUpload(ctx context.Context, req *uipb.RequestPackageUploadRequest) (*uipb.RequestPackageUploadResponse, error)
	DeletePackage(ctx context.Context, req *uipb.DeletePackageRequest) (*emptypb.Empty, error)
}

// PackageService is the gRPC handler for cloud.v1.api.ui.PackageService. Pure
// transport: trace the call and delegate to svc.
type PackageService struct {
	uipb.UnimplementedPackageServiceServer
	*tracing.Entity
	svc PackageActions
}

var _ uipb.PackageServiceServer = (*PackageService)(nil)

func NewPackageService(logger *xlog.Logger, svc PackageActions) *PackageService {
	return &PackageService{
		Entity: tracing.NewEntity(logger.AppendName("PackageService")),
		svc:    svc,
	}
}

func (s *PackageService) ListPackages(ctx context.Context, req *uipb.ListPackagesRequest) (*models.Package_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListPackages",
		func(ctx context.Context, _ trace.Span) (*models.Package_List, error) {
			return s.svc.ListPackages(ctx, req)
		})
}

func (s *PackageService) RequestPackageUpload(ctx context.Context, req *uipb.RequestPackageUploadRequest) (*uipb.RequestPackageUploadResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RequestPackageUpload",
		func(ctx context.Context, _ trace.Span) (*uipb.RequestPackageUploadResponse, error) {
			return s.svc.RequestPackageUpload(ctx, req)
		})
}

func (s *PackageService) DeletePackage(ctx context.Context, req *uipb.DeletePackageRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeletePackage",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.DeletePackage(ctx, req)
		})
}
