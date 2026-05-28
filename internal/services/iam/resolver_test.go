package iam

import (
	"context"
	"errors"
	"testing"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"go.uber.org/mock/gomock"
)

func TestResolver_EffectivePermissions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	memberships := NewMockMembershipReader(ctrl)
	roles := NewMockRoleReader(ctrl)
	resolver := NewResolver(memberships, roles)
	ctx := context.Background()

	t.Run("NotFound", func(t *testing.T) {
		memberships.EXPECT().GetByAccountTenant(ctx, "a1", "t1").Return(nil, derrors.ErrNotFound)
		perms, err := resolver.EffectivePermissions(ctx, "a1", "t1")
		if err != nil || perms != nil {
			t.Error("expected nil, nil")
		}
	})

	t.Run("Success", func(t *testing.T) {
		m := &iam.Membership{RoleIds: []string{"r1"}}
		memberships.EXPECT().GetByAccountTenant(ctx, "a1", "t1").Return(m, nil)
		roles.EXPECT().GetMany(ctx, []string{"r1"}).Return([]*iam.Role{
			{Permissions: []*iam.Permission{{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_READ}}},
		}, nil)
		perms, err := resolver.EffectivePermissions(ctx, "a1", "t1")
		if err != nil {
			t.Fatal(err)
		}
		if len(perms) != 1 {
			t.Error("expected 1 perm")
		}
	})

	t.Run("Error", func(t *testing.T) {
		memberships.EXPECT().GetByAccountTenant(ctx, "a1", "t1").Return(nil, errors.New("err"))
		_, err := resolver.EffectivePermissions(ctx, "a1", "t1")
		if err == nil {
			t.Error("expected error")
		}
	})
}
