package connect

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// refreshCookieName is the HttpOnly cookie carrying the refresh token. The
// browser auto-sends it on Refresh + Logout requests; JavaScript never sees it.
const refreshCookieName = "stroppy_refresh"

// refreshCookiePath scopes the cookie to the auth-service procedures so it is
// not attached to every RPC.
const refreshCookiePath = "/cloud.v1.iam.AuthService/"

// IAMHandler implements AuthServiceHandler, UserServiceHandler, and TenantServiceHandler.
type IAMHandler struct {
	svc          *iam.Service
	refreshTTL   time.Duration
	cookieSecure bool
}

// NewIAMHandler wires the IAM service into Connect handlers. refreshTTL +
// cookieSecure shape the HttpOnly refresh cookie issued on Login/Refresh.
func NewIAMHandler(svc *iam.Service, refreshTTL time.Duration, cookieSecure bool) *IAMHandler {
	return &IAMHandler{svc: svc, refreshTTL: refreshTTL, cookieSecure: cookieSecure}
}

// readRefreshFromRequest reads the refresh token first from the body, then
// falls back to the HttpOnly cookie. Frontend clients send empty body + cookie;
// CLI/SDK clients send body explicitly.
func (h *IAMHandler) readRefreshFromRequest(headers http.Header, bodyToken string) string {
	if bodyToken != "" {
		return bodyToken
	}
	// Cookie header parsing via a synthetic *http.Request.
	r := &http.Request{Header: headers}
	if c, err := r.Cookie(refreshCookieName); err == nil {
		return c.Value
	}
	return ""
}

// setRefreshCookie attaches the HttpOnly cookie to the response.
func (h *IAMHandler) setRefreshCookie(headers http.Header, value string) {
	c := &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.refreshTTL.Seconds()),
	}
	headers.Add("Set-Cookie", c.String())
}

// clearRefreshCookie issues a Max-Age=0 cookie so the browser drops it.
func (h *IAMHandler) clearRefreshCookie(headers http.Header) {
	c := &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}
	headers.Add("Set-Cookie", c.String())
}

// ─── AuthService ────────────────────────────────────────────────────────────

func (h *IAMHandler) Login(ctx context.Context, req *connect.Request[iampb.LoginRequest]) (*connect.Response[iampb.LoginResponse], error) {
	pair, err := h.svc.Login(ctx, req.Msg.GetEmail(), req.Msg.GetPassword())
	if err != nil {
		return nil, err
	}
	resp := connect.NewResponse(&iampb.LoginResponse{Tokens: pair})
	h.setRefreshCookie(resp.Header(), pair.GetRefreshToken())
	return resp, nil
}

func (h *IAMHandler) RefreshTokens(ctx context.Context, req *connect.Request[iampb.RefreshTokenRequest]) (*connect.Response[iampb.RefreshTokenResponse], error) {
	tok := h.readRefreshFromRequest(req.Header(), req.Msg.GetRefreshToken())
	if tok == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("refresh token missing"))
	}
	pair, err := h.svc.RefreshTokens(ctx, tok)
	if err != nil {
		return nil, err
	}
	resp := connect.NewResponse(&iampb.RefreshTokenResponse{Tokens: pair})
	h.setRefreshCookie(resp.Header(), pair.GetRefreshToken())
	return resp, nil
}

func (h *IAMHandler) Logout(ctx context.Context, req *connect.Request[iampb.LogoutRequest]) (*connect.Response[emptypb.Empty], error) {
	tok := h.readRefreshFromRequest(req.Header(), req.Msg.GetRefreshToken())
	if tok != "" {
		if err := h.svc.Logout(ctx, tok); err != nil {
			return nil, err
		}
	}
	resp := connect.NewResponse(&emptypb.Empty{})
	h.clearRefreshCookie(resp.Header())
	return resp, nil
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
