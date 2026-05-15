package connect

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// IAMHandler implements AuthServiceHandler, UserServiceHandler, and TenantServiceHandler.
type IAMHandler struct{ svc *iam.Service }

func NewIAMHandler(svc *iam.Service) *IAMHandler { return &IAMHandler{svc: svc} }

// ─── AuthService ────────────────────────────────────────────────────────────

func (h *IAMHandler) Login(ctx context.Context, req *connect.Request[iampb.LoginRequest]) (*connect.Response[iampb.LoginResponse], error) {
	pair, err := h.svc.Login(ctx, req.Msg.GetEmail(), req.Msg.GetPassword())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.LoginResponse{Tokens: pair}), nil
}

func (h *IAMHandler) RefreshTokens(ctx context.Context, req *connect.Request[iampb.RefreshTokenRequest]) (*connect.Response[iampb.RefreshTokenResponse], error) {
	pair, err := h.svc.RefreshTokens(ctx, req.Msg.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.RefreshTokenResponse{Tokens: pair}), nil
}

func (h *IAMHandler) Logout(ctx context.Context, req *connect.Request[iampb.LogoutRequest]) (*connect.Response[emptypb.Empty], error) {
	if err := h.svc.Logout(ctx, req.Msg.GetRefreshToken()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// ─── UserService ─────────────────────────────────────────────────────────────

func (h *IAMHandler) CreateUser(ctx context.Context, req *connect.Request[iampb.CreateUserRequest]) (*connect.Response[iampb.User], error) {
	u, err := h.svc.CreateUser(ctx, req.Msg.GetUser(), req.Msg.GetPassword())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
}

func (h *IAMHandler) UpdateUser(_ context.Context, _ *connect.Request[iampb.UpdateUserRequest]) (*connect.Response[iampb.User], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("UpdateUser not implemented"))
}

func (h *IAMHandler) DeleteUser(_ context.Context, _ *connect.Request[iampb.UserId]) (*connect.Response[iampb.User], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("DeleteUser not implemented"))
}

func (h *IAMHandler) UpdatePassword(_ context.Context, _ *connect.Request[iampb.UpdatePasswordRequest]) (*connect.Response[iampb.User], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("UpdatePassword not implemented"))
}

func (h *IAMHandler) Me(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[iampb.User], error) {
	userID := middleware.UserFromCtx(ctx)
	if userID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	u, err := h.svc.GetUserByID(ctx, &iampb.UserId{Value: userID})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
}

// ─── TenantService ───────────────────────────────────────────────────────────

func (h *IAMHandler) CreateTenant(ctx context.Context, req *connect.Request[iampb.CreateTenantRequest]) (*connect.Response[iampb.Tenant], error) {
	userID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	t, err := h.svc.CreateTenant(ctx, req.Msg.GetTenant(), userID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(t), nil
}

func (h *IAMHandler) ListMyTenants(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[iampb.Tenant_List], error) {
	userID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	tenants, err := h.svc.ListTenantsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.Tenant_List{Tenants: tenants}), nil
}

func (h *IAMHandler) UpdateTenant(_ context.Context, _ *connect.Request[iampb.UpdateTenantRequest]) (*connect.Response[iampb.Tenant], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("UpdateTenant not implemented"))
}

func (h *IAMHandler) DeleteTenant(_ context.Context, _ *connect.Request[iampb.TenantId]) (*connect.Response[iampb.Tenant], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("DeleteTenant not implemented"))
}
