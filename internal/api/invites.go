package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// ListInvites — pending invites of a tenant.
func (h *Handler) ListInvites(ctx context.Context, params oas.ListInvitesParams) (*oas.ListInvitesOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	invs, err := h.deps.Tenants.Invites(ctx, a, params.Slug)
	if err != nil {
		return nil, err
	}
	out := &oas.ListInvitesOK{Data: make([]oas.Invite, 0, len(invs))}
	for _, inv := range invs {
		v, err := h.inviteOf(ctx, inv)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, v)
	}
	return out, nil
}

// CreateInvite — invite by email.
func (h *Handler) CreateInvite(ctx context.Context, req *oas.CreateInviteReq, params oas.CreateInviteParams) (*oas.Invite, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	inv, err := h.deps.Tenants.Invite(ctx, a, params.Slug, req.Email, tenant.Role(req.Role), req.Message.Or(""))
	if err != nil {
		return nil, err
	}
	v, err := h.inviteOf(ctx, inv)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// RevokeInvite — withdraw.
func (h *Handler) RevokeInvite(ctx context.Context, params oas.RevokeInviteParams) error {
	a, err := actor(ctx)
	if err != nil {
		return err
	}
	return h.deps.Tenants.RevokeInvite(ctx, a, params.Slug, params.ID)
}

// ListMyInvites — invites addressed to the caller.
func (h *Handler) ListMyInvites(ctx context.Context) (*oas.ListMyInvitesOK, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	invs, err := h.deps.Tenants.MyInvites(ctx, a.Email)
	if err != nil {
		return nil, err
	}
	out := &oas.ListMyInvitesOK{Data: make([]oas.Invite, 0, len(invs))}
	for _, inv := range invs {
		v, err := h.inviteOf(ctx, inv)
		if err != nil {
			return nil, err
		}
		out.Data = append(out.Data, v)
	}
	return out, nil
}

// AcceptInvite — join.
func (h *Handler) AcceptInvite(ctx context.Context, params oas.AcceptInviteParams) (*oas.TenantMembership, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.deps.Tenants.Accept(ctx, a, params.ID)
	if err != nil {
		return nil, err
	}
	out := membershipOf(m)
	return &out, nil
}

// DeclineInvite — refuse.
func (h *Handler) DeclineInvite(ctx context.Context, params oas.DeclineInviteParams) error {
	a, err := actor(ctx)
	if err != nil {
		return err
	}
	return h.deps.Tenants.Decline(ctx, a, params.ID)
}

// inviteOf renders an invite with its tenant.
func (h *Handler) inviteOf(ctx context.Context, inv tenant.Invite) (oas.Invite, error) {
	t, err := h.deps.Tenants.ByID(ctx, inv.TenantID)
	if err != nil {
		return oas.Invite{}, err
	}
	out := oas.Invite{
		ID: inv.ID, Tenant: *tenantOf(t), Email: inv.Email, Role: oas.TenantRole(inv.Role),
		Status: oas.InviteStatus(inv.Status), CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt,
	}
	if inv.InvitedBy != nil {
		ref := oas.UserRef{ID: inv.InvitedBy.String()}
		if inv.InviterName != "" {
			ref.DisplayName = oas.NewOptString(inv.InviterName)
		}
		if inv.InviterAvatar != "" {
			ref.Avatar = oas.NewOptString(inv.InviterAvatar)
		}
		out.InvitedBy = oas.NewOptUserRef(ref)
	}
	if inv.Message != "" {
		out.Message = oas.NewOptString(inv.Message)
	}
	return out, nil
}
