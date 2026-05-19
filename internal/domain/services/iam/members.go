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

// RemoveMember soft-deletes a tenant member by ID and returns the previous
// row (pre-delete) for handler response.
func (s *Service) RemoveMember(ctx context.Context, id *iampb.TenantMemberId) (*iampb.TenantMember, error) {
	existing, err := s.GetMember(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := s.memberRepo.Execute(ctx,
		iampb.TenantMembers.Update().
			Set(iampb.TenantMembers.DeletedAt.Set(&now)).
			Where(iampb.TenantMembers.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return existing, nil
}

// GetMember returns a non-deleted tenant member by ID.
func (s *Service) GetMember(ctx context.Context, id *iampb.TenantMemberId) (*iampb.TenantMember, error) {
	m, err := s.memberRepo.QueryRow(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.Id.Eq(id.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

// UpdateMemberRole updates the role of an existing tenant membership.
func (s *Service) UpdateMemberRole(ctx context.Context, id *iampb.TenantMemberId, role iampb.TenantRole) (*iampb.TenantMember, error) {
	now := time.Now()
	if _, err := s.memberRepo.Execute(ctx,
		iampb.TenantMembers.Update().
			Set(
				iampb.TenantMembers.Role.Set(role.String()),
				iampb.TenantMembers.UpdatedAt.Set(now),
			).
			Where(iampb.TenantMembers.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return s.GetMember(ctx, id)
}

// ListMembersByUser returns all non-deleted memberships of a user (every
// tenant they belong to).
func (s *Service) ListMembersByUser(ctx context.Context, userID *iampb.UserId) ([]*iampb.TenantMember, error) {
	return s.memberRepo.Query(ctx,
		iampb.TenantMembers.SelectAll().Where(
			iampb.TenantMembers.UserId.Eq(userID.GetValue()),
			iampb.TenantMembers.DeletedAt.IsNull(),
		),
	)
}
