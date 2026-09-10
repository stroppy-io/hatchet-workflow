package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	iamsdk "github.com/gopherex/iam/pkg/sdk"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/profile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	apitoken "github.com/stroppy-io/stroppy-cloud/internal/domain/token"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
)

/*
THE ACTOR comes ONLY from a verified token: login and registration live in
IAM; the server has no password table and never will.

Two passes, dispatched by token FORM:
  - personal API token `stc_...` — ours, hashed in the DB (lands with the
    tokens area);
  - IAM access token — a person.

Hybrid verification does not see revocations, so the IAM webhook writes
revoked sessions into a denylist the verifier consults; the window is the
project's access_ttl.
*/

// IAMConfig is the `iam` section.
type IAMConfig struct {
	Mode        string `default:"hybrid" mapstructure:"mode"        validate:"oneof=remote local hybrid"`
	BaseURL     string `mapstructure:"base_url"                     validate:"required,url"`
	Credential  string `mapstructure:"credential"`
	ProjectID   string `mapstructure:"project_id"                   validate:"required"`
	Environment string `default:"live"   mapstructure:"environment" validate:"required"`
	Issuer      string `mapstructure:"issuer"`
	Audience    string `mapstructure:"audience"`
	JWKSURL     string `mapstructure:"jwks_url"`
	// ClientID is the SPA's app client (X-Client-Id); served to the SPA
	// through the public config.
	ClientID string `mapstructure:"client_id" validate:"required"`
	// WebhookSigningSecret verifies IAM lifecycle events; empty = the
	// endpoint is not mounted.
	WebhookSigningSecret string `mapstructure:"webhook_signing_secret"`
	WebhookPath          string `default:"/webhooks/iam" mapstructure:"webhook_path" validate:"required"`
	// AccessTTL is the project's access-token lifetime: how long a revoked
	// session stays on the denylist.
	AccessTTL       time.Duration `default:"10m" mapstructure:"access_ttl"`
	JWKSCacheTTLSec int           `default:"3600" mapstructure:"jwks_cache_ttl_sec"`
	WarmTimeout     time.Duration `default:"5s" mapstructure:"warm_timeout"`
}

func (c *IAMConfig) WebhookEnabled() bool { return c.WebhookSigningSecret != "" }

// errUnauthenticated — the token is not recognized. Never leaves the
// process with a reason attached.
var errUnauthenticated = errors.New("token not recognized")

// verifierChain dispatches by token form: `stc_` is ours, anything else
// is IAM's.
type verifierChain struct {
	tokens *apitoken.Service
	iam    *iamVerifier
}

func (v verifierChain) Verify(ctx context.Context, token string) (auth.Actor, error) {
	if token == "" {
		return auth.Actor{}, errUnauthenticated
	}
	if apitoken.IsWire(token) {
		return v.tokens.Verify(ctx, token)
	}
	return v.iam.Verify(ctx, token)
}

// iamVerifier verifies IAM access tokens. The IAM subject IS the profile
// id: no numbering of our own, it would diverge from IAM on day one. The
// profile is ensured on every verified call (cheap upsert) so the rest of
// the server can assume it exists.
type iamVerifier struct {
	auth     iamsdk.Authenticator
	users    *iam.Users
	denylist *repositories.IAMRepo
	profiles *profile.Service
	tenants  *tenant.Service
	log      *xlog.Logger
}

// newIAMVerifier builds the verifier and probes IAM. A failed warm-up does
// NOT disable verification: hybrid caches JWKS as soon as IAM answers.
func newIAMVerifier(
	ctx context.Context,
	cfg *IAMConfig,
	denylist *repositories.IAMRepo,
	profiles *profile.Service,
	tenants *tenant.Service,
	log *xlog.Logger,
) (v *iamVerifier, warmErr, err error) {
	authenticator, err := iamsdk.NewAuthenticator(iamsdk.AuthenticatorConfig{
		Mode:            iamsdk.ValidationMode(cfg.Mode),
		BaseURL:         cfg.BaseURL,
		Credential:      cfg.Credential,
		ProjectID:       cfg.ProjectID,
		Environment:     cfg.Environment,
		Issuer:          cfg.Issuer,
		Audience:        cfg.Audience,
		JWKSURL:         cfg.JWKSURL,
		JWKSCacheTTLSec: cfg.JWKSCacheTTLSec,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("iam authenticator: %w", err)
	}
	warmCtx, cancel := context.WithTimeout(ctx, cfg.WarmTimeout)
	defer cancel()
	if err := iamsdk.Warm(warmCtx, authenticator); err != nil {
		warmErr = fmt.Errorf("iam warm: %w", err)
	}
	return &iamVerifier{
		auth: authenticator, users: iam.NewUsers(cfg.BaseURL, cfg.ClientID, cfg.Environment),
		denylist: denylist, profiles: profiles, tenants: tenants, log: log,
	}, warmErr, nil
}

func (v *iamVerifier) Verify(ctx context.Context, token string) (auth.Actor, error) {
	principal, err := v.auth.Authenticate(ctx, token)
	if err != nil {
		// A rejected token is normal client state; a misconfigured verifier
		// lands here too, so keep it at debug — first place to look.
		v.log.Ctx().Debug(ctx, "iam: token rejected", xlog.ErrorCause(err))
		return auth.Actor{}, errUnauthenticated
	}
	user, err := uuid.Parse(principal.UserID)
	if err != nil {
		v.log.Ctx().Error(ctx, "iam: subject is not a uuid", xlog.String("subject", principal.UserID))
		return auth.Actor{}, errUnauthenticated
	}
	if principal.SessionID != "" {
		denied, err := v.denylist.SessionDenied(ctx, principal.SessionID)
		if err != nil {
			return auth.Actor{}, err
		}
		if denied {
			return auth.Actor{}, errUnauthenticated
		}
	}
	email, _ := principal.Claims["email"].(string) //nolint:errcheck // absent claim = empty email
	p, created, err := v.profiles.Ensure(ctx, user, email)
	if err != nil {
		return auth.Actor{}, err
	}
	if created || p.Email == "" {
		p = v.firstSight(ctx, token, user, p)
	}
	return auth.Actor{UserID: user, SessionID: principal.SessionID, Email: p.Email}, nil
}

// firstSight fills the profile from IAM users/me (the token carries no
// email) and applies invites waiting for that email. Failures degrade:
// the actor is still valid, the next call tries again.
func (v *iamVerifier) firstSight(ctx context.Context, token string, user uuid.UUID, p profile.Profile) profile.Profile {
	me, err := v.users.Me(ctx, token)
	if err != nil {
		v.log.Ctx().Warn(ctx, "iam: users/me failed", xlog.ErrorCause(err))
		return p
	}
	seeded, err := v.profiles.Seed(ctx, user, me.Email, me.Profile.Name, me.Profile.AvatarURL)
	if err != nil {
		v.log.Ctx().Warn(ctx, "profile: seed failed", xlog.ErrorCause(err))
		return p
	}
	if err := v.tenants.ApplyPending(ctx, user, seeded.Email); err != nil {
		v.log.Ctx().Warn(ctx, "invites: apply pending failed", xlog.ErrorCause(err))
	}
	return seeded
}
