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
