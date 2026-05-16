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
