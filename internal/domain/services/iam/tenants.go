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
