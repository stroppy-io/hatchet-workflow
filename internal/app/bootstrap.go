package app

import (
	"context"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/catalog"
)

// Bootstrap idempotently seeds the control plane before it starts serving:
//  1. a root admin account + its tenant + OWNER membership (so there is someone to
//     log in as), created from cfg only when no admin account exists yet;
//  2. the settings scaffold — one empty SettingsItem per known Key under the root
//     tenant, so the settings UI has rows to fill;
//  3. the system database presets (is_system) under the root tenant.
//
// Every step is check-then-insert, so running it on every start is safe.
func Bootstrap(ctx context.Context, executor exec.DB, txm tx.Trm, cfg *Config, logger *xlog.Logger) error {
	log := logger.AppendName("bootstrap")
	accounts := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Accounts.Table, executor), models.AccountConverter)
	tenants := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Tenants.Table, executor), models.TenantConverter)
	members := repository.NewProtoRepository(
		repository.NewScannerRepository(models.TenantMembers.Table, executor), models.TenantMemberConverter)
	settings := repository.NewProtoRepository(
		repository.NewScannerRepository(models.SettingsItems.Table, executor), models.SettingsItemConverter)
	presets := repository.NewProtoRepository(
		repository.NewScannerRepository(models.Presets.Table, executor), models.PresetConverter)

	_, err := tx.DoReadCommittedRet(ctx, txm, func(ctx context.Context) (struct{}, error) {
		accountID, tenantID, err := ensureRootAdmin(ctx, accounts, tenants, members, cfg, log)
		if err != nil {
			return struct{}{}, err
		}
		if err := ensureSettingsScaffold(ctx, settings, tenantID, log); err != nil {
			return struct{}{}, err
		}
		if err := ensureSystemPresets(ctx, presets, accountID, tenantID, log); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

type accountRepo = *repository.ProtoRepository[models.AccountAlias, models.AccountColumnAlias, *models.AccountScanner, *models.Account]
type tenantRepo = *repository.ProtoRepository[models.TenantAlias, models.TenantColumnAlias, *models.TenantScanner, *models.Tenant]
type memberRepo = *repository.ProtoRepository[models.TenantMemberAlias, models.TenantMemberColumnAlias, *models.TenantMemberScanner, *models.TenantMember]
type settingsRepo = *repository.ProtoRepository[models.SettingsItemAlias, models.SettingsItemColumnAlias, *models.SettingsItemScanner, *models.SettingsItem]
type presetRepo = *repository.ProtoRepository[models.PresetAlias, models.PresetColumnAlias, *models.PresetScanner, *models.Preset]

// ensureRootAdmin returns the root admin account + tenant ids, creating them (admin
// account, its tenant, an OWNER membership) when no admin account exists.
func ensureRootAdmin(ctx context.Context, accounts accountRepo, tenants tenantRepo, members memberRepo, cfg *Config, log *xlog.Logger) (*models.AccountId, *models.TenantId, error) {
	admins, err := accounts.Query(ctx, models.Accounts.SelectAll().Where(
		models.Accounts.IsAdmin.Eq(true), models.Accounts.DeletedAt.IsNull()).Limit(1))
	if err != nil {
		return nil, nil, err
	}

	var accountID *models.AccountId
	if len(admins) > 0 {
		accountID = &models.AccountId{Value: admins[0].GetEntity().GetId().GetValue()}
	} else {
		hash, err := domainauth.HashPassword(cfg.RootAdminPassword)
		if err != nil {
			return nil, nil, err
		}
		acc := &models.Account{Entity: ids.NewEntity(), Email: cfg.RootAdminEmail, Nickname: "root", IsAdmin: true}
		sc := acc.IntoPlain()
		sc.PasswordHash = hash
		if _, err := accounts.Scanner().Execute(ctx, models.Accounts.Insert().From(sc.AllSetters()...)); err != nil {
			return nil, nil, err
		}
		accountID = &models.AccountId{Value: acc.GetEntity().GetId().GetValue()}
		log.Info("seeded root admin account", xlog.String("email", cfg.RootAdminEmail))
	}

	// The root tenant is the one owned by the admin account.
	owned, err := tenants.Query(ctx, models.Tenants.SelectAll().Where(
		models.Tenants.OwnerAccountId.Eq(accountID.GetValue()), models.Tenants.DeletedAt.IsNull()).Limit(1))
	if err != nil {
		return nil, nil, err
	}
	if len(owned) > 0 {
		return accountID, &models.TenantId{Value: owned[0].GetEntity().GetId().GetValue()}, nil
	}

	tenant := &models.Tenant{Entity: ids.NewEntity(), OwnerAccountId: accountID, Name: ptr(cfg.RootTenantName)}
	if _, err := tenants.Execute(ctx, models.Tenants.Insert().From(tenant.IntoPlain().AllSetters()...)); err != nil {
		return nil, nil, err
	}
	tenantID := &models.TenantId{Value: tenant.GetEntity().GetId().GetValue()}
	member := &models.TenantMember{Entity: ids.NewEntity(), TenantId: tenantID, AccountId: accountID, Role: models.TenantMember_ROLE_OWNER}
	if _, err := members.Execute(ctx, models.TenantMembers.Insert().From(member.IntoPlain().AllSetters()...)); err != nil {
		return nil, nil, err
	}
	log.Info("seeded root tenant", xlog.String("tenant_id", tenantID.GetValue()))
	return accountID, tenantID, nil
}

// ensureSettingsScaffold inserts one empty SettingsItem per known Key (Yandex Cloud
// partition) under the tenant, when absent.
func ensureSettingsScaffold(ctx context.Context, settings settingsRepo, tenantID *models.TenantId, log *xlog.Logger) error {
	existing, err := settings.Query(ctx, models.SettingsItems.SelectAll().Where(
		models.SettingsItems.TenantId.Eq(tenantID.GetValue())))
	if err != nil {
		return err
	}
	have := make(map[models.SettingsItem_Key]bool, len(existing))
	for _, it := range existing {
		have[it.GetKey()] = true
	}
	seeded := 0
	for v := range models.SettingsItem_Key_name {
		key := models.SettingsItem_Key(v)
		if key == models.SettingsItem_KEY_UNSPECIFIED || have[key] {
			continue
		}
		item := &models.SettingsItem{
			Id:       &models.SettingsItemId{Value: ids.New()},
			TenantId: tenantID,
			Part:     models.SettingsItem_PART_YANDEX_CLOUD,
			Key:      key,
			Value:    &models.SettingsItem_Value{Value: &models.SettingsItem_Value_StringValue{StringValue: ""}},
		}
		e := ids.NewEntity()
		item.Timestamps = e.GetTimestamps()
		if _, err := settings.Execute(ctx, models.SettingsItems.Insert().From(item.IntoPlain().AllSetters()...)); err != nil {
			return err
		}
		seeded++
	}
	if seeded > 0 {
		log.Info("seeded settings scaffold", xlog.Int("count", seeded))
	}
	return nil
}

// ensureSystemPresets inserts the catalog's system database presets under the tenant,
// matched by name, when absent.
func ensureSystemPresets(ctx context.Context, presets presetRepo, accountID *models.AccountId, tenantID *models.TenantId, log *xlog.Logger) error {
	existing, err := presets.Query(ctx, models.Presets.SelectAll().Where(
		models.Presets.TenantId.Eq(tenantID.GetValue()),
		models.Presets.IsSystem.Eq(true)))
	if err != nil {
		return err
	}
	have := make(map[string]bool, len(existing))
	for _, p := range existing {
		have[p.GetName()] = true
	}
	seeded := 0
	for _, sp := range catalog.SystemDatabasePresets() {
		if have[sp.Name] {
			continue
		}
		preset := &models.Preset{
			Entity:   ids.NewEntity(),
			Owned:    &models.Own{OwnerAccountId: accountID, TenantId: tenantID},
			Name:     ptr(sp.Name),
			Kind:     models.Preset_KIND_DATABASE,
			IsSystem: true,
			Preset:   &models.Preset_DatabasePreset{DatabasePreset: sp.DB},
		}
		if sp.Description != "" {
			preset.Description = ptr(sp.Description)
		}
		scanner := preset.IntoPlain()
		nilEmptyPresetJSONB(scanner)
		if _, err := presets.Execute(ctx, models.Presets.Insert().From(scanner.AllSetters()...)); err != nil {
			return err
		}
		seeded++
	}
	if seeded > 0 {
		log.Info("seeded system database presets", xlog.Int("count", seeded))
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// nilEmptyPresetJSONB nulls the unused preset oneof JSONB columns so the not-null
// jsonb columns are not written as empty strings (mirrors catalog's insert path).
func nilEmptyPresetJSONB(s *models.PresetScanner) {
	if len(s.PresetWorkloadPreset) == 0 {
		s.PresetWorkloadPreset = nil
	}
	if len(s.PresetDatabasePreset) == 0 {
		s.PresetDatabasePreset = nil
	}
	if len(s.PresetTestPreset) == 0 {
		s.PresetTestPreset = nil
	}
}
