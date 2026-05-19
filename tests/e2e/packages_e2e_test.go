//go:build e2e

package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

func customPackage(name string, kind catalogpb.Database_Kind, version string) *catalogpb.Package {
	return &catalogpb.Package{
		Identity:  &commonpb.Identity{Name: name},
		DbKind:    kind,
		DbVersion: version,
		IsBuiltin: false,
		Source: &catalogpb.Package_PackageSource{
			Source: &catalogpb.Package_PackageSource_Apt{
				Apt: &catalogpb.Package_AptSource{AptPackages: []string{name}},
			},
		},
	}
}

func TestE2E_Packages_BuiltinSeededOnTenantCreate(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pkg-tenant")
	cli := env.WithTenant(tnt.GetValue())

	lst, err := cli.Package.ListPackages(ctx, connect.NewRequest(&catalogpb.ListPackagesRequest{TenantId: tnt}))
	require.NoError(t, err)
	builtinCount := 0
	for _, p := range lst.Msg.GetPackages() {
		if p.GetIsBuiltin() {
			builtinCount++
		}
	}
	require.Greater(t, builtinCount, 0, "expected at least one builtin package seeded")
}

func TestE2E_Packages_CreateCustomPackage_GetList(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pkg-create")
	cli := env.WithTenant(tnt.GetValue())

	created, err := cli.Package.CreatePackage(ctx, connect.NewRequest(&catalogpb.CreatePackageRequest{
		Package: customPackage("custom-pg", catalogpb.Database_DATABASE_KIND_POSTGRES, "16"),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, created.Msg.GetId().GetValue())

	got, err := cli.Package.GetPackage(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	require.Equal(t, "custom-pg", got.Msg.GetIdentity().GetName())

	lst, err := cli.Package.ListPackages(ctx, connect.NewRequest(&catalogpb.ListPackagesRequest{TenantId: tnt}))
	require.NoError(t, err)
	require.NotEmpty(t, lst.Msg.GetPackages())
}

func TestE2E_Packages_UpdatePackage(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pkg-update")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.Package.CreatePackage(ctx, connect.NewRequest(&catalogpb.CreatePackageRequest{
		Package: customPackage("upd-pg", catalogpb.Database_DATABASE_KIND_POSTGRES, "15"),
	}))
	require.NoError(t, err)
	pkg := created.Msg
	pkg.DbVersion = "16"
	pkg.Identity = &commonpb.Identity{Name: "upd-pg-renamed"}
	upd, err := cli.Package.UpdatePackage(ctx, connect.NewRequest(&catalogpb.UpdatePackageRequest{Package: pkg}))
	require.NoError(t, err)
	require.Equal(t, "16", upd.Msg.GetDbVersion())
	require.Equal(t, "upd-pg-renamed", upd.Msg.GetIdentity().GetName())
}

func TestE2E_Packages_DeleteSoftDelete(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pkg-del")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.Package.CreatePackage(ctx, connect.NewRequest(&catalogpb.CreatePackageRequest{
		Package: customPackage("del-pg", catalogpb.Database_DATABASE_KIND_POSTGRES, "15"),
	}))
	require.NoError(t, err)
	id := created.Msg.GetId().GetValue()
	_, err = cli.Package.DeletePackage(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)

	lst, err := cli.Package.ListPackages(ctx, connect.NewRequest(&catalogpb.ListPackagesRequest{TenantId: tnt}))
	require.NoError(t, err)
	for _, p := range lst.Msg.GetPackages() {
		require.NotEqual(t, id, p.GetId().GetValue())
	}
}

func TestE2E_Packages_ListFiltersByKind(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pkg-filter")
	cli := env.WithTenant(tnt.GetValue())

	kind := catalogpb.Database_DATABASE_KIND_POSTGRES
	lst, err := cli.Package.ListPackages(ctx, connect.NewRequest(&catalogpb.ListPackagesRequest{
		TenantId: tnt, DbKind: &kind,
	}))
	require.NoError(t, err)
	for _, p := range lst.Msg.GetPackages() {
		require.Equal(t, catalogpb.Database_DATABASE_KIND_POSTGRES, p.GetDbKind(),
			"all packages must have postgres kind when filtered")
	}
}

func TestE2E_Packages_ClonePackage_DupesPayload(t *testing.T) {
	env := newEnv(t, "[]", "[]")
	ctx := context.Background()
	tnt := env.SeedTenant(t, "pkg-clone")
	cli := env.WithTenant(tnt.GetValue())
	created, err := cli.Package.CreatePackage(ctx, connect.NewRequest(&catalogpb.CreatePackageRequest{
		Package: customPackage("clone-me", catalogpb.Database_DATABASE_KIND_POSTGRES, "15"),
	}))
	require.NoError(t, err)
	cloned, err := cli.Package.ClonePackage(ctx, connect.NewRequest(created.Msg.GetId()))
	require.NoError(t, err)
	require.NotEqual(t, created.Msg.GetId().GetValue(), cloned.Msg.GetId().GetValue())
	require.Equal(t, created.Msg.GetDbKind(), cloned.Msg.GetDbKind())
	require.Equal(t, created.Msg.GetDbVersion(), cloned.Msg.GetDbVersion())
}
