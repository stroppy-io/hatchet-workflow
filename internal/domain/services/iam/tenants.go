package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// CreateTenant creates a tenant and adds the owner as TENANT_ROLE_OWNER in one tx.
func (s *Service) CreateTenant(ctx context.Context, tenant *iampb.Tenant, ownerID *iampb.UserId) (*iampb.Tenant, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateTenant",
		func(ctx context.Context, _ trace.Span) (*iampb.Tenant, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.Tenant, error) {
					now := time.Now()
					tenant.Id = &iampb.TenantId{Value: ids.New()}
					tenant.Timestamps = &commonpb.Timestamps{
						CreatedAt: timestamppb.New(now),
						UpdatedAt: timestamppb.New(now),
					}
					tScanner := tenant.IntoPlain()
					if tScanner.Label == nil {
						tScanner.Label = []string{}
					}
					if _, err := s.tenantRepo.Execute(ctx,
						iampb.Tenants.Insert().From(tScanner.AllSetters()...),
					); err != nil {
						return nil, err
					}

					member := &iampb.TenantMember{
						Id:       &iampb.TenantMemberId{Value: ids.New()},
						TenantId: tenant.GetId(),
						UserId:   ownerID,
						Role:     iampb.TenantRole_TENANT_ROLE_OWNER,
						Timestamps: &commonpb.Timestamps{
							CreatedAt: timestamppb.New(now),
							UpdatedAt: timestamppb.New(now),
						},
					}
					mScanner := member.IntoPlain()
					if _, err := s.memberRepo.Execute(ctx,
						iampb.TenantMembers.Insert().From(mScanner.AllSetters()...),
					); err != nil {
						return nil, err
					}

					_ = s.events.Publish(ctx, eventing.Event{
						Topic:   eventing.TopicTenantCreated,
						Payload: eventing.TenantCreated{TenantID: tenant.GetId().GetValue()},
					})
					_ = s.events.Publish(ctx, eventing.Event{
						Topic: eventing.TopicMemberAdded,
						Payload: eventing.MemberAdded{
							UserID:   ownerID.GetValue(),
							TenantID: tenant.GetId().GetValue(),
							Role:     iampb.TenantRole_TENANT_ROLE_OWNER.String(),
						},
					})
					return tenant, nil
				})
		})
}

// GetTenantByID retrieves a tenant by ID, returning NotFound if absent.
func (s *Service) GetTenantByID(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error) {
	t, err := s.tenantRepo.QueryRow(ctx,
		iampb.Tenants.SelectAll().Where(
			iampb.Tenants.Id.Eq(id.GetValue()),
			iampb.Tenants.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("tenant", id.GetValue()))
		}
		return nil, err
	}
	return t, nil
}

// ListAllTenants returns all non-deleted tenants (platform-admin view).
func (s *Service) ListAllTenants(ctx context.Context) ([]*iampb.Tenant, error) {
	return s.tenantRepo.Query(ctx,
		iampb.Tenants.SelectAll().Where(iampb.Tenants.DeletedAt.IsNull()),
	)
}

// UpdateTenant patches mutable identity fields (name, description). The tenant
// id and timestamps are immutable. Returns the post-update row.
func (s *Service) UpdateTenant(ctx context.Context, tenant *iampb.Tenant) (*iampb.Tenant, error) {
	if tenant.GetId() == nil || tenant.GetId().GetValue() == "" {
		return nil, domainerr.InvalidArgument(domainerr.FieldViolation("id", "is required"))
	}
	existing, err := s.GetTenantByID(ctx, tenant.GetId())
	if err != nil {
		return nil, err
	}
	if tenant.GetIdentity() != nil {
		if name := tenant.GetIdentity().GetName(); name != "" {
			existing.Identity.Name = name
		}
		if desc := tenant.GetIdentity().GetDescription(); desc != "" {
			d := desc
			existing.Identity.Description = &d
		}
	}
	now := time.Now()
	var descPtr *string
	if existing.GetIdentity() != nil && existing.GetIdentity().Description != nil {
		descPtr = existing.GetIdentity().Description
	}
	if _, err := s.tenantRepo.Execute(ctx,
		iampb.Tenants.Update().
			Set(
				iampb.Tenants.Name.Set(existing.GetIdentity().GetName()),
				iampb.Tenants.Description.Set(descPtr),
				iampb.Tenants.UpdatedAt.Set(now),
			).
			Where(iampb.Tenants.Id.Eq(tenant.GetId().GetValue())),
	); err != nil {
		return nil, err
	}
	return s.GetTenantByID(ctx, tenant.GetId())
}

// DeleteTenant soft-deletes a tenant (sets deleted_at). For hard-delete use
// DeleteTenantHard via the admin surface.
func (s *Service) DeleteTenant(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error) {
	existing, err := s.GetTenantByID(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := s.tenantRepo.Execute(ctx,
		iampb.Tenants.Update().
			Set(iampb.Tenants.DeletedAt.Set(&now)).
			Where(iampb.Tenants.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return existing, nil
}

// DeleteTenantHard hard-deletes a tenant by ID (cascades on FKs via DB constraints).
func (s *Service) DeleteTenantHard(ctx context.Context, id *iampb.TenantId) (*iampb.Tenant, error) {
	t, err := s.GetTenantByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.tenantRepo.Execute(ctx,
		iampb.Tenants.Delete().Where(iampb.Tenants.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return t, nil
}

// ListTenantsForUser returns tenants where user is a member.
func (s *Service) ListTenantsForUser(ctx context.Context, userID *iampb.UserId) ([]*iampb.Tenant, error) {
	members, err := s.memberRepo.Query(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.UserId.Eq(userID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		return nil, err
	}
	out := make([]*iampb.Tenant, 0, len(members))
	for _, m := range members {
		t, err := s.GetTenantByID(ctx, m.GetTenantId())
		if err != nil {
			if errors.Is(err, domainerr.NotFound()) {
				continue // tenant deleted; skip
			}
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
