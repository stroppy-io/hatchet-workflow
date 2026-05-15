package iam

import (
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Service owns IAM tables and exposes business methods consumed by Connect handlers.
type Service struct {
	*tracing.Entity

	userRepo    *repository.ProtoRepository[iampb.UserAlias, iampb.UserColumnAlias, *iampb.UserScanner, *iampb.User]
	tenantRepo  *repository.ProtoRepository[iampb.TenantAlias, iampb.TenantColumnAlias, *iampb.TenantScanner, *iampb.Tenant]
	memberRepo  *repository.ProtoRepository[iampb.TenantMemberAlias, iampb.TenantMemberColumnAlias, *iampb.TenantMemberScanner, *iampb.TenantMember]
	refreshRepo *repository.ProtoRepository[iampb.RefreshTokenAlias, iampb.RefreshTokenColumnAlias, *iampb.RefreshTokenScanner, *iampb.RefreshToken]
	tokenRepo   *repository.ProtoRepository[iampb.ApiTokenAlias, iampb.ApiTokenColumnAlias, *iampb.ApiTokenScanner, *iampb.ApiToken]

	txManager pgtx.TxManager
	events    eventing.Bus
	cfg       configurator.AuthConfig
	jwtSecret []byte
}

func New(executor exec.DB, txManager pgtx.TxManager, events eventing.Bus, cfg configurator.AuthConfig, jwtSecret []byte) *Service {
	return &Service{
		Entity: tracing.NewEntity("iam.Service"),
		userRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.Users.Table, executor),
			iampb.UserConverter,
		),
		tenantRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.Tenants.Table, executor),
			iampb.TenantConverter,
		),
		memberRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.TenantMembers.Table, executor),
			iampb.TenantMemberConverter,
		),
		refreshRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.RefreshTokens.Table, executor),
			iampb.RefreshTokenConverter,
		),
		tokenRepo: repository.NewProtoRepository(
			repository.NewScannerRepository(iampb.ApiTokens.Table, executor),
			iampb.ApiTokenConverter,
		),
		txManager: txManager,
		events:    events,
		cfg:       cfg,
		jwtSecret: jwtSecret,
	}
}
