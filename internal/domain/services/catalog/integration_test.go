package catalog_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
)

func TestCreateAndListDatabasePresets(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "c@e.com", Nickname: "c"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())

	preset := &catalogpb.DatabasePreset{
		Identity: &commonpb.Identity{Name: "pg-default"},
		Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
	}
	created, err := f.Catalog.CreateDatabasePreset(ctx, tn.GetId(), u.GetId(), preset)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	list, err := f.Catalog.ListDatabasePresets(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "pg-default", list[0].GetIdentity().GetName())
}

func TestPackageCRUDWithoutBinary(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "p@e.com", Nickname: "p"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())

	pkg := &catalogpb.Package{
		Identity:  &commonpb.Identity{Name: "postgres-15"},
		DbKind:    catalogpb.Database_DATABASE_KIND_POSTGRES,
		DbVersion: "15",
		Source: &catalogpb.Package_PackageSource{
			Source: &catalogpb.Package_PackageSource_Apt{
				Apt: &catalogpb.Package_AptSource{
					AptPackages: []string{"postgresql-15"},
				},
			},
		},
	}
	created, err := f.Catalog.CreatePackage(ctx, tn.GetId(), u.GetId(), pkg)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	list, err := f.Catalog.ListPackages(ctx, tn.GetId(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, list)
}

func TestCreateAndListWorkloadPresets(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "w@e.com", Nickname: "w"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())

	preset := &catalogpb.WorkloadPreset{
		Identity: &commonpb.Identity{Name: "tpcc-default"},
		Workload: &catalogpb.Workload{},
	}
	created, err := f.Catalog.CreateWorkloadPreset(ctx, tn.GetId(), u.GetId(), preset)
	require.NoError(t, err)
	require.NotEmpty(t, created.GetId().GetValue())

	list, err := f.Catalog.ListWorkloadPresets(ctx, tn.GetId())
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestSettingsSetAndGet(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "s@e.com", Nickname: "s"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T"}}, u.GetId())

	val := &catalogpb.SettingsItem_Value{Value: &catalogpb.SettingsItem_Value_StringValue{StringValue: "yc-token-123"}}
	row, err := f.Catalog.SetSetting(ctx, tn.GetId(),
		catalogpb.SettingsItem_PART_YANDEX_CLOUD,
		catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
		val,
	)
	require.NoError(t, err)
	require.NotEmpty(t, row.GetId().GetValue())

	got, err := f.Catalog.GetSetting(ctx, tn.GetId(),
		catalogpb.SettingsItem_PART_YANDEX_CLOUD,
		catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN,
	)
	require.NoError(t, err)
	require.Equal(t, "yc-token-123", got.GetValue().GetStringValue())
}

// TestDatabasePresets_TenantIsolation: presets created under tenant A must
// not surface in ListDatabasePresets for tenant B.
func TestDatabasePresets_TenantIsolation(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	uA, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "ia@e.com", Nickname: "ia"}, "P@ss1234!")
	tnA, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "TA"}}, uA.GetId())
	uB, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "ib@e.com", Nickname: "ib"}, "P@ss1234!")
	tnB, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "TB"}}, uB.GetId())

	_, err := f.Catalog.CreateDatabasePreset(ctx, tnA.GetId(), uA.GetId(), &catalogpb.DatabasePreset{
		Identity: &commonpb.Identity{Name: "a-only-preset"},
		Database: &catalogpb.Database{Kind: catalogpb.Database_DATABASE_KIND_POSTGRES},
	})
	require.NoError(t, err)

	listA, err := f.Catalog.ListDatabasePresets(ctx, tnA.GetId())
	require.NoError(t, err)
	require.Len(t, listA, 1)

	listB, err := f.Catalog.ListDatabasePresets(ctx, tnB.GetId())
	require.NoError(t, err)
	require.Empty(t, listB, "Tenant B must not see Tenant A's database presets")
}

func TestWorkloadPresets_TenantIsolation(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	uA, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "wa@e.com", Nickname: "wa"}, "P@ss1234!")
	tnA, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "WA"}}, uA.GetId())
	uB, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "wb@e.com", Nickname: "wb"}, "P@ss1234!")
	tnB, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "WB"}}, uB.GetId())

	_, err := f.Catalog.CreateWorkloadPreset(ctx, tnA.GetId(), uA.GetId(), &catalogpb.WorkloadPreset{
		Identity: &commonpb.Identity{Name: "a-only-wl"},
		Workload: &catalogpb.Workload{},
	})
	require.NoError(t, err)

	listB, err := f.Catalog.ListWorkloadPresets(ctx, tnB.GetId())
	require.NoError(t, err)
	require.Empty(t, listB, "Tenant B must not see Tenant A's workload presets")
}

func TestSettings_TenantIsolation(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	uA, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "sa@e.com", Nickname: "sa"}, "P@ss1234!")
	tnA, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "SA"}}, uA.GetId())
	uB, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "sb@e.com", Nickname: "sb"}, "P@ss1234!")
	tnB, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "SB"}}, uB.GetId())

	valA := &catalogpb.SettingsItem_Value{Value: &catalogpb.SettingsItem_Value_StringValue{StringValue: "tenant-A"}}
	_, err := f.Catalog.SetSetting(ctx, tnA.GetId(),
		catalogpb.SettingsItem_PART_YANDEX_CLOUD,
		catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN, valA)
	require.NoError(t, err)

	// Same key in tenant B must NOT see A's value.
	gotB, err := f.Catalog.GetSetting(ctx, tnB.GetId(),
		catalogpb.SettingsItem_PART_YANDEX_CLOUD,
		catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN)
	if err == nil {
		require.NotEqual(t, "tenant-A", gotB.GetValue().GetStringValue(), "tenant B must not read tenant A's setting")
	}
}

func TestSettingsUpsertReplacesValue(t *testing.T) {
	f := fixture.NewCatalog(t)
	ctx := context.Background()

	u, _ := f.IAM.CreateUser(ctx, &iampb.User{Email: "s2@e.com", Nickname: "s2"}, "P@ss1234!")
	tn, _ := f.IAM.CreateTenant(ctx, &iampb.Tenant{Identity: &commonpb.Identity{Name: "T2"}}, u.GetId())

	v1 := &catalogpb.SettingsItem_Value{Value: &catalogpb.SettingsItem_Value_StringValue{StringValue: "v1"}}
	r1, err := f.Catalog.SetSetting(ctx, tn.GetId(), catalogpb.SettingsItem_PART_YANDEX_CLOUD, catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN, v1)
	require.NoError(t, err)

	v2 := &catalogpb.SettingsItem_Value{Value: &catalogpb.SettingsItem_Value_StringValue{StringValue: "v2"}}
	r2, err := f.Catalog.SetSetting(ctx, tn.GetId(), catalogpb.SettingsItem_PART_YANDEX_CLOUD, catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN, v2)
	require.NoError(t, err)
	require.Equal(t, r1.GetId().GetValue(), r2.GetId().GetValue(), "upsert must reuse existing row")

	got, err := f.Catalog.GetSetting(ctx, tn.GetId(), catalogpb.SettingsItem_PART_YANDEX_CLOUD, catalogpb.SettingsItem_KEY_YANDEX_CLOUD_TOKEN)
	require.NoError(t, err)
	require.Equal(t, "v2", got.GetValue().GetStringValue())
}
