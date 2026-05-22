package admin

import (
	"context"

	"github.com/gopherex/xlog"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// AccountAdminActions is the dependency the AccountAdminService is built on:
// platform-level (is_admin) account management. internal/services implements it.
type AccountAdminActions interface {
	ListAccounts(ctx context.Context, req *adminpb.ListAccountsRequest) (*adminpb.ListAccountsResponse, error)
	CreateAccount(ctx context.Context, req *adminpb.CreateAccountRequest) (*models.Account, error)
	UpdateAccount(ctx context.Context, req *adminpb.UpdateAccountRequest) (*emptypb.Empty, error)
	DeleteAccount(ctx context.Context, id *models.AccountId) (*emptypb.Empty, error)
	UpdatePassword(ctx context.Context, req *adminpb.UpdatePasswordRequest) (*emptypb.Empty, error)
}

// AccountAdminService is the gRPC handler for cloud.v1.api.admin.AccountAdminService.
// It is pure transport: trace the call and delegate to svc. Transport adaptation
// (gRPC/connect) is wired at the application level.
type AccountAdminService struct {
	adminpb.UnimplementedAccountAdminServiceServer
	*tracing.Entity
	svc AccountAdminActions
}

var _ adminpb.AccountAdminServiceServer = (*AccountAdminService)(nil)

func NewAccountAdminService(logger *xlog.Logger, svc AccountAdminActions) *AccountAdminService {
	return &AccountAdminService{
		Entity: tracing.NewEntity(logger.AppendName("AccountAdminService")),
		svc:    svc,
	}
}

func (s *AccountAdminService) ListAccounts(ctx context.Context, req *adminpb.ListAccountsRequest) (*adminpb.ListAccountsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListAccounts",
		func(ctx context.Context, _ trace.Span) (*adminpb.ListAccountsResponse, error) {
			return s.svc.ListAccounts(ctx, req)
		})
}

func (s *AccountAdminService) CreateAccount(ctx context.Context, req *adminpb.CreateAccountRequest) (*models.Account, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateAccount",
		func(ctx context.Context, _ trace.Span) (*models.Account, error) {
			return s.svc.CreateAccount(ctx, req)
		})
}

func (s *AccountAdminService) UpdateAccount(ctx context.Context, req *adminpb.UpdateAccountRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateAccount",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.UpdateAccount(ctx, req)
		})
}

func (s *AccountAdminService) DeleteAccount(ctx context.Context, id *models.AccountId) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteAccount",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.DeleteAccount(ctx, id)
		})
}

func (s *AccountAdminService) UpdatePassword(ctx context.Context, req *adminpb.UpdatePasswordRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdatePassword",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			return s.svc.UpdatePassword(ctx, req)
		})
}
