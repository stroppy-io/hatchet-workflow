package identity

import (
	"context"
	"errors"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// PlatformSettingsGetter is the minimal stored read the settings/gates adapters
// need. The gormstore *SettingsRepo satisfies it (its Get(ctx) returns the
// singleton PlatformSettings, or derrors.ErrNotFound on a brand-new install).
type PlatformSettingsGetter interface {
	Get(ctx context.Context) (*api.PlatformSettings, error)
}

// SettingsReader implements iamsvc.SettingsReader: it loads the singleton
// PlatformSettings, treating a brand-new install (no row) as the zero-value
// (locked-down) settings rather than an error.
type SettingsReader struct {
	settings PlatformSettingsGetter
}

var _ iamsvc.SettingsReader = (*SettingsReader)(nil)

// NewSettingsReader builds the reader over a stored settings getter (inject the
// gormstore *SettingsRepo at wiring time).
func NewSettingsReader(settings PlatformSettingsGetter) *SettingsReader {
	return &SettingsReader{settings: settings}
}

// PlatformSettings returns the singleton settings. A not-yet-created row yields
// an empty PlatformSettings (every allow_* flag false) so gated flows stay
// locked down until an admin opts in.
func (r *SettingsReader) PlatformSettings(ctx context.Context) (*api.PlatformSettings, error) {
	s, err := r.settings.Get(ctx)
	if errors.Is(err, derrors.ErrNotFound) {
		return &api.PlatformSettings{}, nil
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Gates implements iamsvc.PlatformGates over a SettingsReader, exposing the
// global allow_* process flags as domain decisions. The flags are polarised so
// the zero value is locked-down.
type Gates struct {
	settings iamsvc.SettingsReader
}

var _ iamsvc.PlatformGates = (*Gates)(nil)

// NewGates builds the gates over a settings reader.
func NewGates(settings iamsvc.SettingsReader) *Gates {
	return &Gates{settings: settings}
}

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
