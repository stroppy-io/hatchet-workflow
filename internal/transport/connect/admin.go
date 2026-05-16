package connect

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	adminsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/admin"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

// ─── AdminHandler ─────────────────────────────────────────────────────────────

// AdminHandler implements adminconnect.AdminServiceHandler.
type AdminHandler struct {
	svc *adminsvc.AdminService
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(svc *adminsvc.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListAllTenants(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[iampb.Tenant_List], error) {
	list, err := h.svc.ListAllTenants(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *AdminHandler) CreateTenant(ctx context.Context, req *connect.Request[adminpb.AdminCreateTenantRequest]) (*connect.Response[iampb.Tenant], error) {
	tenant, err := h.svc.CreateTenant(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(tenant), nil
}

func (h *AdminHandler) DeleteTenantHard(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[iampb.Tenant], error) {
	tenant, err := h.svc.DeleteTenantHard(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(tenant), nil
}

func (h *AdminHandler) ListAllUsers(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[iampb.User_List], error) {
	list, err := h.svc.ListAllUsers(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *AdminHandler) CreateUser(ctx context.Context, req *connect.Request[adminpb.AdminCreateUserRequest]) (*connect.Response[iampb.User], error) {
	user, err := h.svc.CreateUser(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(user), nil
}

func (h *AdminHandler) DeleteUser(ctx context.Context, req *connect.Request[iampb.UserId]) (*connect.Response[iampb.User], error) {
	user, err := h.svc.DeleteUser(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(user), nil
}

func (h *AdminHandler) ResetUserPassword(ctx context.Context, req *connect.Request[adminpb.AdminResetPasswordRequest]) (*connect.Response[iampb.User], error) {
	user, err := h.svc.ResetUserPassword(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(user), nil
}

// ─── BinaryCacheAdminHandler ─────────────────────────────────────────────────

// BinaryCacheAdminHandler implements adminconnect.BinaryCacheAdminServiceHandler.
type BinaryCacheAdminHandler struct {
	svc *adminsvc.BinaryCacheAdminService
}

// NewBinaryCacheAdminHandler constructs a BinaryCacheAdminHandler.
func NewBinaryCacheAdminHandler(svc *adminsvc.BinaryCacheAdminService) *BinaryCacheAdminHandler {
	return &BinaryCacheAdminHandler{svc: svc}
}

func (h *BinaryCacheAdminHandler) Prewarm(ctx context.Context, req *connect.Request[adminpb.PrewarmRequest]) (*connect.Response[opspb.BinaryArtifact_List], error) {
	list, err := h.svc.Prewarm(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *BinaryCacheAdminHandler) GetArtifact(ctx context.Context, req *connect.Request[opspb.BinaryArtifactId]) (*connect.Response[opspb.BinaryArtifact], error) {
	art, err := h.svc.GetArtifact(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(art), nil
}

func (h *BinaryCacheAdminHandler) ListArtifacts(ctx context.Context, req *connect.Request[adminpb.ListArtifactsRequest]) (*connect.Response[opspb.BinaryArtifact_List], error) {
	list, err := h.svc.ListArtifacts(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(list), nil
}

func (h *BinaryCacheAdminHandler) EvictArtifact(ctx context.Context, req *connect.Request[opspb.BinaryArtifactId]) (*connect.Response[opspb.BinaryArtifact], error) {
	art, err := h.svc.EvictArtifact(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(art), nil
}
