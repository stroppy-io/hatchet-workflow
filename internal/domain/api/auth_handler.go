package api

import (
	"context"

	"connectrpc.com/connect"
	apiv1 "github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type authHandler struct {
	adminUser     string
	adminPassword string
	jwtMgr        *JWTManager
}

func NewAuthHandler(adminUser, adminPassword string, jwtMgr *JWTManager) *authHandler {
	return &authHandler{
		adminUser:     adminUser,
		adminPassword: adminPassword,
		jwtMgr:        jwtMgr,
	}
}

func (h *authHandler) Login(_ context.Context, req *connect.Request[apiv1.LoginRequest]) (*connect.Response[apiv1.LoginResponse], error) {
	if req.Msg.GetUsername() != h.adminUser || req.Msg.GetPassword() != h.adminPassword {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	accessToken, refreshToken, expiresAt, err := h.jwtMgr.GenerateTokens(req.Msg.GetUsername())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&apiv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
	}), nil
}

func (h *authHandler) RefreshToken(_ context.Context, req *connect.Request[apiv1.RefreshTokenRequest]) (*connect.Response[apiv1.RefreshTokenResponse], error) {
	claims, err := h.jwtMgr.ValidateRefreshToken(req.Msg.GetRefreshToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	accessToken, refreshToken, expiresAt, err := h.jwtMgr.GenerateTokens(claims.Username)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&apiv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    timestamppb.New(expiresAt),
	}), nil
}
