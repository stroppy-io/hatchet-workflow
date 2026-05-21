package auth

import (
	"github.com/gopherex/xlog"
	"github.com/valkey-io/valkey-go"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/api/middleware"
	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// AuthService implements the ui AuthService RPCs and the auth middleware's
// Authenticator. It reads accounts/api-tokens from Postgres (ratel) and stores
// refresh sessions in Valkey.
type AuthService struct {
	*tracing.Entity
	accountRepo *repository.ProtoRepository[
		models.AccountAlias,
		models.AccountColumnAlias,
		*models.AccountScanner,
		*models.Account,
	]
	apiTokenRepo *repository.ProtoRepository[
		models.ApiTokenAlias,
		models.ApiTokenColumnAlias,
		*models.ApiTokenScanner,
		*models.ApiToken,
	]
	valkey valkey.Client
	signer *domainauth.Signer
	cfg    Config
}

var (
	_ uiapi.AuthActions        = (*AuthService)(nil)
	_ middleware.Authenticator = (*AuthService)(nil)
)

// New builds an AuthService. executor is the ratel DB handle (tx-aware), vk the
// Valkey client for refresh sessions.
func New(logger *xlog.Logger, executor exec.DB, vk valkey.Client, cfg Config) *AuthService {
	return &AuthService{
		Entity: tracing.NewEntity(logger.AppendName("AuthService")),
		accountRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Accounts.Table, executor),
			models.AccountConverter,
		),
		apiTokenRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(models.ApiTokens.Table, executor),
			models.ApiTokenConverter,
		),
		valkey: vk,
		signer: domainauth.NewSigner(cfg.JWTSecret()),
		cfg:    cfg,
	}
}
