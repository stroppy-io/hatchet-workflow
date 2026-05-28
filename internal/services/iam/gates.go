package iam

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
)

/*
	Gates exposes the global process gates (PlatformSettings allow_* flags) as
	domain decisions. The flags are polarised so the zero value is the safe,
	locked-down state; a missing/empty settings row denies the gated flows for
	everyone but platform admins (the admin bypass lives in the handler).
*/

// SettingsReader loads the singleton PlatformSettings.
type SettingsReader interface {
	PlatformSettings(ctx context.Context) (*api.PlatformSettings, error)
}

type Gates struct {
	settings SettingsReader
}

func NewGates(settings SettingsReader) *Gates {
	return &Gates{settings: settings}
}

var _ PlatformGates = (*Gates)(nil)

func (g *Gates) SelfRegistrationAllowed(ctx context.Context) (bool, error) {
	s, err := g.settings.PlatformSettings(ctx)
	if err != nil {
		return false, err
	}
	return s.GetAllowSelfRegistration(), nil
}

func (g *Gates) MemberTenantCreationAllowed(ctx context.Context) (bool, error) {
	s, err := g.settings.PlatformSettings(ctx)
	if err != nil {
		return false, err
	}
	return s.GetAllowMemberTenantCreation(), nil
}
