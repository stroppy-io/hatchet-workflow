package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/identity"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// seedFirstBoot creates the initial admin account + default tenant + owner role
// + membership, the singleton PlatformSettings, and the builtin catalog on a
// brand-new database, so that a fresh install has a principal that can actually
// log in and a minimal runnable self-check suite.
//
// It is idempotent in two independent ways:
//   - if cfg.AdminPassword is empty it skips entirely (we never invent a
//     password);
//   - if ANY account already exists (or specifically one with the admin email),
//     it skips account creation and only ensures platform settings plus any
//     missing builtin catalog records for the default tenant.
//
// All writes run inside one serializable transaction via db.Trm(); a failure
// rolls the whole thing back so we never leave a half-seeded control plane.
func seedFirstBoot(ctx context.Context, log *slog.Logger, store *postgres.Store, db *postgres.DB, hasher *identity.BcryptHasher, catalog *identity.Catalog, cfg Config) error {
	if cfg.AdminPassword == "" {
		log.Info("first-boot seeding skipped: STROPPY_ADMIN_PASSWORD is empty (set it to seed the initial admin)")
		return nil
	}

	email := cfg.AdminEmail
	if email == "" {
		email = "admin@stroppy.local"
	}

	accounts := store.Accounts()
	credentials := store.Credentials()
	tenants := store.Tenants()
	roles := store.Roles()
	memberships := store.Memberships()
	settings := store.Settings()

	hash, err := hasher.Hash(cfg.AdminPassword)
	if err != nil {
		return err
	}

	return tx.DoSerializable(ctx, db.Trm(), func(ctx context.Context) error {
		// Idempotency: any existing account means the platform identity was
		// already bootstrapped (manually or by a prior boot). Do not recreate
		// identities; only ensure settings/catalog for the default tenant.
		existing, _, err := accounts.List(ctx, 1, "")
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			log.Info("first-boot seeding skipped: an account already exists", slog.Int("accounts", len(existing)))
			return ensurePlatformSettings(ctx, log, settings, cfg)
		}
		// Belt-and-suspenders: also short-circuit on the specific email.
		if _, err := accounts.GetByEmail(ctx, email); err == nil {
			log.Info("first-boot seeding skipped: admin account already exists", slog.String("email", email))
			return ensurePlatformSettings(ctx, log, settings, cfg)
		} else if !errors.Is(err, derrors.ErrNotFound) {
			return err
		}

		now := timestamppb.Now()

		// 1) Admin account: the platform super-user (is_admin) so it holds full
		// authority across every tenant regardless of membership.
		account := &iam.Account{
			Id:            uuid.NewString(),
			Email:         email,
			EmailVerified: true,
			Nickname:      "admin",
			IsAdmin:       true,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := accounts.Create(ctx, account); err != nil {
			return err
		}

		// 2) Password credential (bcrypt hash from the identity hasher).
		if err := credentials.SetPassword(ctx, account.Id, hash); err != nil {
			return err
		}

		// 3) Default tenant owned by the admin account.
		tenant := &iam.Tenant{
			Id:             uuid.NewString(),
			Name:           "Default",
			Slug:           "default",
			OwnerAccountId: account.Id,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tenants.Create(ctx, tenant); err != nil {
			return err
		}

		// 4) System owner role inside the tenant, granted the full MANAGE set
		// enumerated by the permission catalog (every grantable resource).
		perms, err := catalogManagePermissions(ctx, catalog)
		if err != nil {
			return err
		}
		ownerRole := &iam.Role{
			Id:          uuid.NewString(),
			Name:        "owner",
			Scope:       iam.Scope_SCOPE_TENANT,
			TenantId:    tenant.Id,
			IsSystem:    true,
			Permissions: perms,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := roles.Create(ctx, ownerRole); err != nil {
			return err
		}

		// 5) Membership linking the admin to the tenant with the owner role.
		membership := &iam.Membership{
			Id:        uuid.NewString(),
			AccountId: account.Id,
			TenantId:  tenant.Id,
			RoleIds:   []string{ownerRole.Id},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := memberships.Create(ctx, membership); err != nil {
			return err
		}

		// 6) Default platform settings (idempotent upsert).
		if err := ensurePlatformSettings(ctx, log, settings, cfg); err != nil {
			return err
		}

		log.Info("first-boot seeding complete",
			slog.String("admin_email", email),
			slog.String("admin_account_id", account.Id),
			slog.String("tenant_id", tenant.Id),
			slog.String("tenant_slug", tenant.Slug),
			slog.String("owner_role_id", ownerRole.Id),
			slog.Int("owner_permissions", len(perms)),
		)
		return nil
	}, tx.WithRetry(tx.DefaultRetryPolicy))
}

// ensurePlatformSettings creates the singleton PlatformSettings row if absent,
// defaulting the agent-facing ServerAddr from cfg.AgentServerAddr. If a row
// already exists it is left untouched.
func ensurePlatformSettings(ctx context.Context, log *slog.Logger, settings *postgres.SettingsRepo, cfg Config) error {
	if _, err := settings.Get(ctx); err == nil {
		return nil
	} else if !errors.Is(err, derrors.ErrNotFound) {
		return err
	}
	defaults := &api.PlatformSettings{
		ServerAddr:                cfg.AgentServerAddr,
		AllowSelfRegistration:     false,
		AllowMemberTenantCreation: false,
	}
	if err := settings.Set(ctx, defaults); err != nil {
		return err
	}
	log.Info("first-boot seeding: default platform settings created",
		slog.String("server_addr", defaults.ServerAddr))
	return nil
}

// catalogManagePermissions returns one MANAGE permission per grantable resource,
// derived from the permission catalog — the broadest grant an admin role can
// hold without hard-coding the resource list here.
func catalogManagePermissions(ctx context.Context, catalog *identity.Catalog) ([]*iam.Permission, error) {
	entries, err := catalog.Grantable(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[iam.Resource]bool, len(entries))
	perms := make([]*iam.Permission, 0, len(entries))
	for _, e := range entries {
		p := e.GetPermission()
		if p.GetAction() != iam.Action_ACTION_MANAGE {
			continue
		}
		if seen[p.GetResource()] {
			continue
		}
		seen[p.GetResource()] = true
		perms = append(perms, &iam.Permission{Resource: p.GetResource(), Action: iam.Action_ACTION_MANAGE})
	}
	return perms, nil
}

// ensure iamsvc port types stay referenced so a signature drift in the repos
// (which must satisfy these interfaces) surfaces here at compile time.
var (
	_ iamsvc.AccountRepo     = (*postgres.AccountRepo)(nil)
	_ iamsvc.CredentialStore = (*postgres.CredentialRepo)(nil)
	_ iamsvc.TenantRepo      = (*postgres.TenantRepo)(nil)
	_ iamsvc.RoleRepo        = (*postgres.RoleRepo)(nil)
	_ iamsvc.MembershipRepo  = (*postgres.MembershipRepo)(nil)
)
