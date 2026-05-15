package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// AddMember adds a user to a tenant with the given role.
func (s *Service) AddMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, role iampb.TenantRole) (*iampb.TenantMember, error) {
	now := time.Now()
	m := &iampb.TenantMember{
		Id:       &iampb.TenantMemberId{Value: ids.New()},
		TenantId: tenantID,
		UserId:   userID,
		Role:     role,
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
	}
	scanner := m.IntoPlain()
	if _, err := s.memberRepo.Execute(ctx,
		iampb.TenantMembers.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, err
	}
	_ = s.events.Publish(ctx, eventing.Event{
		Topic: eventing.TopicMemberAdded,
		Payload: eventing.MemberAdded{
			UserID:   userID.GetValue(),
			TenantID: tenantID.GetValue(),
			Role:     role.String(),
		},
	})
	return m, nil
}

// HasTenantRole returns true iff user has a non-deleted membership in tenant with role >= min.
func (s *Service) HasTenantRole(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId, min iampb.TenantRole) (bool, error) {
	m, err := s.memberRepo.QueryRow(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.UserId.Eq(userID.GetValue()),
			iampb.TenantMembers.TenantId.Eq(tenantID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return m.GetRole() >= min, nil
}

// HasTenantMember returns true iff user has at least VIEWER role in the tenant.
func (s *Service) HasTenantMember(ctx context.Context, userID *iampb.UserId, tenantID *iampb.TenantId) (bool, error) {
	return s.HasTenantRole(ctx, userID, tenantID, iampb.TenantRole_TENANT_ROLE_VIEWER)
}

// ListMembers returns all non-deleted members of a tenant.
func (s *Service) ListMembers(ctx context.Context, tenantID *iampb.TenantId) ([]*iampb.TenantMember, error) {
	return s.memberRepo.Query(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.TenantId.Eq(tenantID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
}

// RemoveMember soft-deletes a tenant member by ID.
func (s *Service) RemoveMember(ctx context.Context, id *iampb.TenantMemberId) error {
	now := time.Now()
	_, err := s.memberRepo.Execute(ctx,
		iampb.TenantMembers.Update().
			Set(iampb.TenantMembers.DeletedAt.Set(&now)).
			Where(iampb.TenantMembers.Id.Eq(id.GetValue())),
	)
	return err
}
