package iam

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"go.uber.org/mock/gomock"
)

func TestGates_SelfRegistrationAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settings := NewMockSettingsReader(ctrl)
	gates := NewGates(settings)
	ctx := context.Background()

	settings.EXPECT().PlatformSettings(ctx).Return(&api.PlatformSettings{AllowSelfRegistration: true}, nil)
	allowed, err := gates.SelfRegistrationAllowed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Error("expected true")
	}
}

func TestGates_MemberTenantCreationAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	settings := NewMockSettingsReader(ctrl)
	gates := NewGates(settings)
	ctx := context.Background()

	settings.EXPECT().PlatformSettings(ctx).Return(&api.PlatformSettings{AllowMemberTenantCreation: true}, nil)
	allowed, err := gates.MemberTenantCreationAllowed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Error("expected true")
	}
}
