package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/identity"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
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
			if err := ensurePlatformSettings(ctx, log, settings, cfg); err != nil {
				return err
			}
			return seedExistingDefaultTenantCatalog(ctx, log, store, tenants)
		}
		// Belt-and-suspenders: also short-circuit on the specific email.
		if _, err := accounts.GetByEmail(ctx, email); err == nil {
			log.Info("first-boot seeding skipped: admin account already exists", slog.String("email", email))
			if err := ensurePlatformSettings(ctx, log, settings, cfg); err != nil {
				return err
			}
			return seedExistingDefaultTenantCatalog(ctx, log, store, tenants)
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

		// 7) Builtin/system preset catalog and the default self-check suite.
		if err := seedBuiltinCatalog(ctx, log, store, tenant.Id, account.Id); err != nil {
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

func seedExistingDefaultTenantCatalog(ctx context.Context, log *slog.Logger, store *postgres.Store, tenants *postgres.TenantRepo) error {
	tenant, err := tenants.GetBySlug(ctx, "default")
	if errors.Is(err, derrors.ErrNotFound) {
		log.Info("first-boot seeding: default tenant not found, builtin catalog skipped")
		return nil
	}
	if err != nil {
		return err
	}
	return seedBuiltinCatalog(ctx, log, store, tenant.GetId(), tenant.GetOwnerAccountId())
}

func seedBuiltinCatalog(ctx context.Context, log *slog.Logger, store *postgres.Store, tenantID, authorID string) error {
	dbPresets, err := seedDatabasePresets(ctx, log, store.DatabasePresets(), tenantID, authorID)
	if err != nil {
		return err
	}
	workloads, err := seedWorkloadPresets(ctx, log, store.WorkloadPresets(), tenantID, authorID)
	if err != nil {
		return err
	}
	tests, err := seedTestPresets(ctx, log, store.TestPresets(), tenantID, authorID, dbPresets, workloads)
	if err != nil {
		return err
	}
	return seedSuites(ctx, log, store.Suites(), tenantID, authorID, tests)
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

// seedDatabasePresets ensures the builtin/system database presets for the
// tenant, mirroring the catalog `main` shipped. It is idempotent by entity name:
// missing builtin names are inserted, and existing builtin records are
// reconciled with non-destructive catalog metadata such as the builtin package.
func seedDatabasePresets(ctx context.Context, log *slog.Logger, repo *postgres.DatabasePresetRepo, tenantID, authorID string) ([]*models.DatabasePresetRecord, error) {
	existing, _, err := repo.List(ctx, &api.ListDatabasePresetsRequest{TenantId: tenantID}, authorID)
	if err != nil {
		return nil, err
	}
	byName := databasePresetsByName(existing)
	presets := builtinDatabasePresets(tenantID, authorID)
	created := 0
	updated := 0
	out := make([]*models.DatabasePresetRecord, 0, len(presets))
	for _, p := range presets {
		if current := byName[p.GetEntity().GetName()]; current != nil {
			if reconcileBuiltinDatabasePreset(current, p) {
				if err := repo.Update(ctx, current); err != nil {
					return nil, err
				}
				updated++
			}
			out = append(out, current)
			continue
		}
		if err := repo.Create(ctx, p); err != nil {
			return nil, err
		}
		created++
		out = append(out, p)
	}
	log.Info("first-boot seeding: builtin database presets ensured",
		slog.String("tenant_id", tenantID), slog.Int("created", created), slog.Int("updated", updated), slog.Int("catalog", len(out)))
	return out, nil
}

func reconcileBuiltinDatabasePreset(rec, canonical *models.DatabasePresetRecord) bool {
	if rec == nil || rec.GetDatabase() == nil || rec.GetDatabase().GetParams() == nil {
		return false
	}
	next := cloneDatabaseWithBuiltinPackage(rec.GetDatabase())
	description := rec.GetEntity().GetDescription()
	if canonical != nil && canonical.GetDatabase() != nil {
		next = cloneDatabaseWithBuiltinPackage(canonical.GetDatabase())
		description = canonical.GetEntity().GetDescription()
	}
	changed := !proto.Equal(rec.GetDatabase(), next) ||
		rec.GetIsSystem() != true ||
		rec.GetEntity().GetDescription() != description
	if !changed {
		return false
	}
	rec.Database = next
	rec.IsSystem = true
	if entity := rec.GetEntity(); entity != nil {
		entity.Description = description
		entity.IsFavorite = false
		if timings := entity.GetTimings(); timings != nil {
			timings.UpdatedAt = timestamppb.Now()
		}
	}
	return true
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
