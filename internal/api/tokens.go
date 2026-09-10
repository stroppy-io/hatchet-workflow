package api

import (
	"context"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/token"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListMyTokens — personal API tokens.
func (h *Handler) ListMyTokens(ctx context.Context) (*oas.ListMyTokensOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	ts, err := h.deps.Tokens.Mine(ctx, a)
	if err != nil {
		return nil, err
	}
	out := &oas.ListMyTokensOK{Data: make([]oas.ApiToken, 0, len(ts))}
	for _, t := range ts {
		out.Data = append(out.Data, tokenOf(t))
	}
	return out, nil
}

// CreateMyToken — mint a personal token.
func (h *Handler) CreateMyToken(ctx context.Context, req *oas.ApiTokenCreate) (*oas.ApiTokenCreated, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	role := ""
	if v, ok := req.Role.Get(); ok {
		role = string(v)
	}
	c, err := h.deps.Tokens.CreatePersonal(ctx, a, req.Name, req.TenantID, role, expiresOf(req.ExpiresAt))
	if err != nil {
		return nil, err
	}
	return createdOf(c), nil
}

// RevokeMyToken — revoke a personal token.
func (h *Handler) RevokeMyToken(ctx context.Context, params oas.RevokeMyTokenParams) error {
	a, err := actor(ctx)
	if err != nil {
		return err
	}
	return h.deps.Tokens.RevokeMine(ctx, a, params.ID)
}

// ListTenantTokens — service tokens.
func (h *Handler) ListTenantTokens(ctx context.Context, params oas.ListTenantTokensParams) (*oas.ListTenantTokensOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	t, err := h.deps.Tenants.Get(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	ts, err := h.deps.Tokens.ServiceTokens(ctx, a, t.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.ListTenantTokensOK{Data: make([]oas.ApiToken, 0, len(ts))}
	for _, tk := range ts {
		out.Data = append(out.Data, tokenOf(tk))
	}
	return out, nil
}

// CreateTenantToken — mint a service token.
func (h *Handler) CreateTenantToken(ctx context.Context, req *oas.ServiceTokenCreate, params oas.CreateTenantTokenParams) (*oas.ApiTokenCreated, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	t, err := h.deps.Tenants.Get(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	c, err := h.deps.Tokens.CreateService(ctx, a, t.ID, req.Name, string(req.Role), expiresOf(req.ExpiresAt))
	if err != nil {
		return nil, err
	}
	return createdOf(c), nil
}

// RevokeTenantToken — revoke a service token.
func (h *Handler) RevokeTenantToken(ctx context.Context, params oas.RevokeTenantTokenParams) error {
	a, err := actor(ctx)
	if err != nil {
		return err
	}
	t, err := h.deps.Tenants.Get(ctx, a, params.Slug)
	if err != nil {
		return err
	}
	return h.deps.Tokens.RevokeService(ctx, a, t.ID, params.ID)
}

func expiresOf(v oas.OptNilDateTime) *time.Time {
	if t, ok := v.Get(); ok {
		return &t
	}
	return nil
}

func tokenOf(t token.Token) oas.ApiToken {
	out := oas.ApiToken{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, Kind: oas.ApiTokenKind(t.Kind), Role: oas.TenantRole(t.Role),
		Tenant:    oas.NewOptRef(oas.Ref{ID: t.TenantID, Name: oas.NewOptString(t.TenantName)}),
		CreatedAt: t.CreatedAt,
	}
	if t.OwnerID != nil {
		out.Owner = oas.NewOptUserRef(oas.UserRef{ID: t.OwnerID.String()})
	}
	if t.ExpiresAt != nil {
		out.ExpiresAt = oas.NewOptNilDateTime(*t.ExpiresAt)
	}
	if t.LastUsedAt != nil {
		out.LastUsedAt = oas.NewOptNilDateTime(*t.LastUsedAt)
	}
	return out
}

func createdOf(c token.Created) *oas.ApiTokenCreated {
	t := tokenOf(c.Token)
	return &oas.ApiTokenCreated{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, Kind: oas.ApiTokenCreatedKind(t.Kind), Tenant: t.Tenant, Role: t.Role,
		Owner: t.Owner, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt, CreatedAt: t.CreatedAt, Secret: c.Secret,
	}
}
