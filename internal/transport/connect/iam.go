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

func (h *IAMHandler) UpdateUser(ctx context.Context, req *connect.Request[iampb.UpdateUserRequest]) (*connect.Response[iampb.User], error) {
	u, err := h.svc.UpdateUser(ctx, req.Msg.GetUser())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
}

func (h *IAMHandler) DeleteUser(ctx context.Context, req *connect.Request[iampb.UserId]) (*connect.Response[iampb.User], error) {
	u, err := h.svc.DeleteUser(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
}

func (h *IAMHandler) UpdatePassword(ctx context.Context, req *connect.Request[iampb.UpdatePasswordRequest]) (*connect.Response[iampb.User], error) {
	uid := middleware.UserFromCtx(ctx)
	if uid == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing user context"))
	}
	u, err := h.svc.UpdatePassword(ctx, &iampb.UserId{Value: uid}, req.Msg.GetOldPassword(), req.Msg.GetNewPassword(), req.Msg.GetNewPasswordConfirmation())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(u), nil
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

func (h *IAMHandler) UpdateTenant(ctx context.Context, req *connect.Request[iampb.UpdateTenantRequest]) (*connect.Response[iampb.Tenant], error) {
	t, err := h.svc.UpdateTenant(ctx, req.Msg.GetTenant())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(t), nil
}

func (h *IAMHandler) DeleteTenant(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[iampb.Tenant], error) {
	t, err := h.svc.DeleteTenant(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(t), nil
}

// ─── TenantMemberService ─────────────────────────────────────────────────────

func (h *IAMHandler) AddMember(ctx context.Context, req *connect.Request[iampb.AddMemberRequest]) (*connect.Response[iampb.TenantMember], error) {
	m, err := h.svc.AddMember(ctx, req.Msg.GetUserId(), req.Msg.GetTenantId(), req.Msg.GetRole())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(m), nil
}

func (h *IAMHandler) UpdateMemberRole(ctx context.Context, req *connect.Request[iampb.UpdateMemberRoleRequest]) (*connect.Response[iampb.TenantMember], error) {
	m, err := h.svc.UpdateMemberRole(ctx, req.Msg.GetMemberId(), req.Msg.GetRole())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(m), nil
}

func (h *IAMHandler) RemoveMember(ctx context.Context, req *connect.Request[iampb.TenantMemberId]) (*connect.Response[iampb.TenantMember], error) {
	m, err := h.svc.RemoveMember(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(m), nil
}

func (h *IAMHandler) ListByTenant(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[iampb.TenantMember_List], error) {
	items, err := h.svc.ListMembers(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.TenantMember_List{Members: items}), nil
}

func (h *IAMHandler) ListByUser(ctx context.Context, req *connect.Request[iampb.UserId]) (*connect.Response[iampb.TenantMember_List], error) {
	items, err := h.svc.ListMembersByUser(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.TenantMember_List{Members: items}), nil
}

// ─── ApiTokenService ─────────────────────────────────────────────────────────

func (h *IAMHandler) CreateApiToken(ctx context.Context, req *connect.Request[iampb.CreateApiTokenRequest]) (*connect.Response[iampb.CreateApiTokenResponse], error) {
	tok := req.Msg.GetToken()
	tenantID := tok.GetTenantId()
	if tenantID == nil || tenantID.GetValue() == "" {
		tenantID = &iampb.TenantId{Value: middleware.TenantFromCtx(ctx)}
	}
	callerID := &iampb.UserId{Value: middleware.UserFromCtx(ctx)}
	created, plain, err := h.svc.CreateApiToken(ctx, tenantID, callerID, tok.GetName())
	if err != nil {
		return nil, err
	}
	// Copy fields the caller submitted that the service constructor doesn't
	// already take (scopes, description, expires_at). Persist as a follow-up.
	if len(tok.GetScopes()) > 0 || tok.GetExpiresAt() != nil {
		// expires_at update — name/scopes are stored at creation; for scopes
		// we currently rely on the caller adjusting via future RPC, leaving
		// scopes empty here. expires_at can be patched.
		if tok.GetExpiresAt() != nil {
			ts := tok.GetExpiresAt().AsTime()
			updated, uerr := h.svc.UpdateApiTokenExpiry(ctx, created.GetId(), &ts)
			if uerr == nil && updated != nil {
				created = updated
			}
		}
	}
	return connect.NewResponse(&iampb.CreateApiTokenResponse{Token: created, RawToken: plain}), nil
}

func (h *IAMHandler) RevokeApiToken(ctx context.Context, req *connect.Request[iampb.ApiTokenId]) (*connect.Response[iampb.ApiToken], error) {
	tok, err := h.svc.RevokeApiToken(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(tok), nil
}

func (h *IAMHandler) GetApiToken(ctx context.Context, req *connect.Request[iampb.ApiTokenId]) (*connect.Response[iampb.ApiToken], error) {
	tok, err := h.svc.GetApiToken(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(tok), nil
}

func (h *IAMHandler) ListApiTokens(ctx context.Context, req *connect.Request[iampb.TenantId]) (*connect.Response[iampb.ApiToken_List], error) {
	items, err := h.svc.ListApiTokens(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&iampb.ApiToken_List{ApiTokens: items}), nil
}

func (h *IAMHandler) UpdateApiTokenExpiry(ctx context.Context, req *connect.Request[iampb.UpdateApiTokenExpiryRequest]) (*connect.Response[iampb.ApiToken], error) {
	var expires *time.Time
	if req.Msg.GetExpiresAt() != nil {
		t := req.Msg.GetExpiresAt().AsTime()
		expires = &t
	}
	tok, err := h.svc.UpdateApiTokenExpiry(ctx, req.Msg.GetId(), expires)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(tok), nil
}
