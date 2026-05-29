package tenant_settings

// service_test.go: unit tests for TenantSettingsService.
// Covers GetTenantSettings, UpdateTenantSettings, SetTenantProviderSettings,
// plus the validateUniqueProviders and upsertProvider helpers.

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	schemapb "github.com/stroppy-io/schemapb/schemapb"
	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	common "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// newSvc is a test helper to construct a TenantSettingsService with all mocks wired.
func newSvc(
	ctrl *gomock.Controller,
	authn *utils.MockAuthn,
	tenants *utils.MockTenantReader,
	settings *MockTenantSettingsRepo,
	schemas *MockProviderSchemas,
	baker *MockProviderSettingsBaker,
) *TenantSettingsService {
	return NewTenantSettingsService(TenantSettingsDeps{
		Authn:    authn,
		Tenants:  tenants,
		Settings: settings,
		Schemas:  schemas,
		Baker:    baker,
		Tx:       &utils.MockTrm{},
	})
}

// -------- GetTenantSettings --------

func TestGetTenantSettings(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	callerID := "caller-1"

	t.Run("Success_ExistingRow", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, nil, nil)

		existingRecord := &models.TenantSettingsRecord{
			Entity: &common.Entity{TenantId: tenantID, AuthorId: callerID},
		}

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(existingRecord, nil)

		resp, err := svc.GetTenantSettings(ctx, &api.GetTenantSettingsRequest{TenantId: tenantID})
		if err != nil {
			t.Fatalf("GetTenantSettings: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected settings in response")
		}
		if resp.Settings.GetEntity().GetTenantId() != tenantID {
			t.Errorf("expected tenant_id %s, got %s", tenantID, resp.Settings.GetEntity().GetTenantId())
		}
	})

	t.Run("Success_NotFoundSeeded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)

		resp, err := svc.GetTenantSettings(ctx, &api.GetTenantSettingsRequest{TenantId: tenantID})
		if err != nil {
			t.Fatalf("GetTenantSettings seeded: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected seeded settings in response")
		}
		if resp.Settings.GetEntity().GetTenantId() != tenantID {
			t.Errorf("expected tenant_id %s, got %s", tenantID, resp.Settings.GetEntity().GetTenantId())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.GetTenantSettings(ctx, &api.GetTenantSettingsRequest{TenantId: tenantID})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("TenantNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		svc := newSvc(ctrl, authn, tenants, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)

		_, err := svc.GetTenantSettings(ctx, &api.GetTenantSettingsRequest{TenantId: tenantID})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("SettingsRepoError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(nil, errors.New("db error"))

		_, err := svc.GetTenantSettings(ctx, &api.GetTenantSettingsRequest{TenantId: tenantID})
		if err == nil {
			t.Error("expected error from repo")
		}
	})
}

// -------- UpdateTenantSettings --------

func TestUpdateTenantSettings(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	callerID := "caller-1"

	t.Run("Success_NewRow", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, nil, nil)

		req := &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{},
		}

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)
		settings.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateTenantSettings(ctx, req)
		if err != nil {
			t.Fatalf("UpdateTenantSettings: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected settings in response")
		}
	})

	t.Run("Success_ExistingRow", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, nil, nil)

		existing := &models.TenantSettingsRecord{
			Entity: &common.Entity{Id: "row-1", TenantId: tenantID, AuthorId: callerID},
		}
		req := &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{},
		}

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(existing, nil)
		settings.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

		resp, err := svc.UpdateTenantSettings(ctx, req)
		if err != nil {
			t.Fatalf("UpdateTenantSettings existing row: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected settings in response")
		}
	})

	t.Run("NilSettings_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)

		_, err := svc.UpdateTenantSettings(ctx, &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: nil,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("DuplicateProvider_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)

		_, err := svc.UpdateTenantSettings(ctx, &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{
				Providers: []*deployment.ProviderSettings{
					{Provider: deployment.Provider_PROVIDER_DOCKER},
					{Provider: deployment.Provider_PROVIDER_DOCKER},
				},
			},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for duplicate provider, got %v", err)
		}
	})

	t.Run("UnspecifiedProvider_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)

		_, err := svc.UpdateTenantSettings(ctx, &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{
				Providers: []*deployment.ProviderSettings{
					{Provider: deployment.Provider_PROVIDER_UNSPECIFIED},
				},
			},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument for unspecified provider, got %v", err)
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.UpdateTenantSettings(ctx, &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{},
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("TenantNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		svc := newSvc(ctrl, authn, tenants, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)

		_, err := svc.UpdateTenantSettings(ctx, &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{},
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("UpsertError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)
		settings.EXPECT().Upsert(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.UpdateTenantSettings(ctx, &api.UpdateTenantSettingsRequest{
			TenantId: tenantID,
			Settings: &models.TenantSettingsRecord{},
		})
		if err == nil {
			t.Error("expected error from Upsert")
		}
	})
}

// -------- SetTenantProviderSettings --------

func TestSetTenantProviderSettings(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	callerID := "caller-1"
	provider := deployment.Provider_PROVIDER_DOCKER
	filledSettings := &schemapb.Filled{}
	bakedSettings := &schemapb.Baked{}
	schema := &schemapb.Schema{}

	t.Run("Success_NewProvider", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		schemas := NewMockProviderSchemas(ctrl)
		baker := NewMockProviderSettingsBaker(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, schemas, baker)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		schemas.EXPECT().SettingsSchema(ctx, provider).Return(schema, nil)
		baker.EXPECT().Bake(ctx, schema, filledSettings).Return(bakedSettings, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)
		settings.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

		resp, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if err != nil {
			t.Fatalf("SetTenantProviderSettings: %v", err)
		}
		if resp.Settings == nil {
			t.Fatal("expected settings in response")
		}
		if resp.Settings.GetProvider() != provider {
			t.Errorf("expected provider %v, got %v", provider, resp.Settings.GetProvider())
		}
	})

	t.Run("Success_ReplaceExistingProvider", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		schemas := NewMockProviderSchemas(ctrl)
		baker := NewMockProviderSettingsBaker(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, schemas, baker)

		existing := &models.TenantSettingsRecord{
			Entity: &common.Entity{TenantId: tenantID},
			Providers: []*deployment.ProviderSettings{
				{Provider: provider, Settings: &schemapb.Baked{}},
			},
		}

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		schemas.EXPECT().SettingsSchema(ctx, provider).Return(schema, nil)
		baker.EXPECT().Bake(ctx, schema, filledSettings).Return(bakedSettings, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(existing, nil)
		settings.EXPECT().Upsert(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, rec *models.TenantSettingsRecord) error {
			// verify at most one entry per provider
			count := 0
			for _, p := range rec.GetProviders() {
				if p.GetProvider() == provider {
					count++
				}
			}
			if count != 1 {
				t.Errorf("expected exactly one entry for provider %v, got %d", provider, count)
			}
			return nil
		})

		resp, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if err != nil {
			t.Fatalf("SetTenantProviderSettings replace: %v", err)
		}
		if resp.Settings.GetProvider() != provider {
			t.Errorf("expected provider %v, got %v", provider, resp.Settings.GetProvider())
		}
	})

	t.Run("CallerError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(nil, errors.New("no claims"))
		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("expected Unauthenticated, got %v", err)
		}
	})

	t.Run("UnspecifiedProvider_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: deployment.Provider_PROVIDER_UNSPECIFIED,
			Settings: filledSettings,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("NilSettings_InvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		svc := newSvc(ctrl, authn, nil, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: nil,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("TenantNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		svc := newSvc(ctrl, authn, tenants, nil, nil, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)

		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("SchemaNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		schemas := NewMockProviderSchemas(ctrl)
		svc := newSvc(ctrl, authn, tenants, nil, schemas, nil)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		schemas.EXPECT().SettingsSchema(ctx, provider).Return(nil, derrors.ErrNotFound)

		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("BakeInvalidArgument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		schemas := NewMockProviderSchemas(ctrl)
		baker := NewMockProviderSettingsBaker(ctrl)
		svc := newSvc(ctrl, authn, tenants, nil, schemas, baker)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		schemas.EXPECT().SettingsSchema(ctx, provider).Return(schema, nil)
		baker.EXPECT().Bake(ctx, schema, filledSettings).Return(nil, derrors.Invalid("settings", "invalid value"))

		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument from bake, got %v", err)
		}
	})

	t.Run("UpsertError", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authn := utils.NewMockAuthn(ctrl)
		tenants := utils.NewMockTenantReader(ctrl)
		settings := NewMockTenantSettingsRepo(ctrl)
		schemas := NewMockProviderSchemas(ctrl)
		baker := NewMockProviderSettingsBaker(ctrl)
		svc := newSvc(ctrl, authn, tenants, settings, schemas, baker)

		authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: callerID}, nil)
		tenants.EXPECT().Get(ctx, tenantID).Return(&iampb.Tenant{Id: tenantID}, nil)
		schemas.EXPECT().SettingsSchema(ctx, provider).Return(schema, nil)
		baker.EXPECT().Bake(ctx, schema, filledSettings).Return(bakedSettings, nil)
		settings.EXPECT().Get(ctx, tenantID).Return(nil, derrors.ErrNotFound)
		settings.EXPECT().Upsert(ctx, gomock.Any()).Return(errors.New("db error"))

		_, err := svc.SetTenantProviderSettings(ctx, &api.SetTenantProviderSettingsRequest{
			TenantId: tenantID,
			Provider: provider,
			Settings: filledSettings,
		})
		if err == nil {
			t.Error("expected error from Upsert")
		}
	})
}

// -------- validateUniqueProviders --------

func TestValidateUniqueProviders(t *testing.T) {
	t.Run("Empty_OK", func(t *testing.T) {
		if err := validateUniqueProviders(nil); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("UniqueProviders_OK", func(t *testing.T) {
		err := validateUniqueProviders([]*deployment.ProviderSettings{
			{Provider: deployment.Provider_PROVIDER_DOCKER},
			{Provider: deployment.Provider_PROVIDER_YANDEX},
		})
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("DuplicateProvider_InvalidArgument", func(t *testing.T) {
		err := validateUniqueProviders([]*deployment.ProviderSettings{
			{Provider: deployment.Provider_PROVIDER_DOCKER},
			{Provider: deployment.Provider_PROVIDER_DOCKER},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("UnspecifiedProvider_InvalidArgument", func(t *testing.T) {
		err := validateUniqueProviders([]*deployment.ProviderSettings{
			{Provider: deployment.Provider_PROVIDER_UNSPECIFIED},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})
}

// -------- upsertProvider --------

func TestUpsertProvider(t *testing.T) {
	t.Run("AppendNew", func(t *testing.T) {
		cfg := &deployment.ProviderSettings{Provider: deployment.Provider_PROVIDER_DOCKER}
		result := upsertProvider(nil, cfg)
		if len(result) != 1 {
			t.Errorf("expected 1 entry, got %d", len(result))
		}
		if result[0].GetProvider() != deployment.Provider_PROVIDER_DOCKER {
			t.Errorf("unexpected provider %v", result[0].GetProvider())
		}
	})

	t.Run("ReplaceExisting", func(t *testing.T) {
		old := &schemapb.Baked{}
		list := []*deployment.ProviderSettings{
			{Provider: deployment.Provider_PROVIDER_DOCKER, Settings: old},
		}
		newBaked := &schemapb.Baked{}
		cfg := &deployment.ProviderSettings{Provider: deployment.Provider_PROVIDER_DOCKER, Settings: newBaked}
		result := upsertProvider(list, cfg)
		if len(result) != 1 {
			t.Errorf("expected 1 entry after replace, got %d", len(result))
		}
		if result[0].Settings != newBaked {
			t.Error("expected replaced settings")
		}
	})

	t.Run("AppendToExisting", func(t *testing.T) {
		list := []*deployment.ProviderSettings{
			{Provider: deployment.Provider_PROVIDER_DOCKER},
		}
		cfg := &deployment.ProviderSettings{Provider: deployment.Provider_PROVIDER_YANDEX}
		result := upsertProvider(list, cfg)
		if len(result) != 2 {
			t.Errorf("expected 2 entries, got %d", len(result))
		}
	})
}
