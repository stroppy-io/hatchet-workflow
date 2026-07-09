package schema_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	spb "github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/schema"
)

func TestApplyBakedInputs_OverlaysProviderParams(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex", Params: map[string]any{"zone": "ru-central1-a"}}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"db_version": "17",
		"provider":   map[string]any{"replicas": float64(5)},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(spb.Double("replicas")).MustBuild()

	require.NoError(t, schema.ApplyBakedInputs(resolved, baked, paramsSchema))
	require.Equal(t, "ru-central1-a", cluster.Provider.Params["zone"], "existing static param untouched")
	require.Equal(t, float64(5), cluster.Provider.Params["replicas"], "baked provider value overlaid")
}

// TestApplyBakedInputs_RejectsUndeclaredProviderParam is the apply-time half
// of the SP-D T6 strict-fix (form.go's ComposeFormSchema is the bake-time
// half): a Baked carrying a "provider" key the declared params schema never
// mentions must be rejected outright rather than silently copied into
// Provider.Params. This guards the path a Baked reaches dsl.Compile without
// ever having been produced by BakeForm against the exact schema in play
// (e.g. a stale or hand-crafted Baked attached to a Run).
func TestApplyBakedInputs_RejectsUndeclaredProviderParam(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex", Params: map[string]any{"zone": "ru-central1-a"}}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{"replicas": float64(5), "evil_injected_key": "rm -rf /"},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(spb.Double("replicas")).MustBuild()

	err = schema.ApplyBakedInputs(resolved, baked, paramsSchema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "evil_injected_key")
	require.NotContains(t, cluster.Provider.Params, "replicas", "no partial application on reject")
	require.NotContains(t, cluster.Provider.Params, "evil_injected_key")
	require.Equal(t, "ru-central1-a", cluster.Provider.Params["zone"], "pre-existing static param untouched")
}

// TestApplyBakedInputs_NilParamsSchemaRejectsAnyProviderParam covers the
// no-provider-declared case (e.g. the docker builtin, which composes its
// launch form with a nil params schema — see ComposeFormSchema): if baked
// still carries a "provider" object, every key in it is by definition
// undeclared, so the call must reject rather than silently accept it.
func TestApplyBakedInputs_NilParamsSchemaRejectsAnyProviderParam(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "docker"}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{"anything": "goes"},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	err = schema.ApplyBakedInputs(resolved, baked, nil)
	require.Error(t, err)
	require.Empty(t, cluster.Provider.Params)
}

// TestApplyBakedInputs_RejectsUndeclaredNestedObjectKey is the ApplyBakedInputs
// half of the SP-I1 fix: the top-level-only `declared` check (baked_apply.go)
// let an unknown key nested under a fixed-attribute-set object() param
// through, since it only ever inspected paramsSchema.GetFields() (top
// level). A Baked reaching dsl.Compile without going through BakeForm at all
// (see this function's own doc comment) bypasses schemapb.Bake's own strict
// checks entirely, so this defense-in-depth check must recurse the same way.
func TestApplyBakedInputs_RejectsUndeclaredNestedObjectKey(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex"}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{
			"network_settings": map[string]any{
				"cidr":     "10.0.0.0/24",
				"evil_key": "rm -rf /",
			},
		},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(spb.Object("network_settings", spb.Str("cidr")).Strict()).MustBuild()

	err = schema.ApplyBakedInputs(resolved, baked, paramsSchema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "network_settings.evil_key")
	require.Empty(t, cluster.Provider.Params, "no partial application on reject")
}

// TestApplyBakedInputs_AcceptsMapOfObjectArbitraryKeys is the green control
// for the recursion fix: a map(object({...}))-shaped param (schemapb
// fallback: a non-strict Object with zero declared fields, see
// tfvars_schemapb.go's `case "map":`) must keep accepting arbitrary
// user-chosen keys underneath it -- the recursion must only descend into
// (and reject unknown keys within) an object field that itself declares a
// fixed, non-empty field set.
func TestApplyBakedInputs_AcceptsMapOfObjectArbitraryKeys(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex"}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24"},
			},
		},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	// Mirrors the map(T) fallback: an Object field with no declared fields.
	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(spb.Object("subnets")).MustBuild()

	require.NoError(t, schema.ApplyBakedInputs(resolved, baked, paramsSchema))
	subnets, ok := cluster.Provider.Params["subnets"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, subnets, "my-subnet-a")
}

// TestApplyBakedInputs_RejectsUndeclaredMapValueKey is the schemapb v1.6.0
// Map-kind half of the config-injection fix: a map(object({...}))-shaped
// param now derives to a schemapb Map with a Strict value schema
// (tfvars_schemapb.go's `case "map":`), so unknownKeys (baked_apply.go) must
// recurse into every map VALUE against that value schema -- an unknown key
// inside a map value must be rejected, at a path naming the map key.
func TestApplyBakedInputs_RejectsUndeclaredMapValueKey(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex"}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24", "evil_key": "rm -rf /"},
			},
		},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(spb.Map("subnets", spb.Str("cidr")).Strict()).MustBuild()

	err = schema.ApplyBakedInputs(resolved, baked, paramsSchema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subnets.my-subnet-a.evil_key")
	require.Empty(t, cluster.Provider.Params, "no partial application on reject")
}

// TestApplyBakedInputs_AcceptsMapKindArbitraryKeys is the green control for
// the Map-kind recursion: an arbitrary, user-chosen map key (subnet name)
// must still be accepted -- only what's declared missing INSIDE a map value
// is rejected, never the map key itself.
func TestApplyBakedInputs_AcceptsMapKindArbitraryKeys(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex"}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{
			"subnets": map[string]any{
				"my-subnet-a": map[string]any{"cidr": "10.0.1.0/24"},
				"my-subnet-b": map[string]any{"cidr": "10.0.2.0/24"},
			},
		},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(spb.Map("subnets", spb.Str("cidr")).Strict()).MustBuild()

	require.NoError(t, schema.ApplyBakedInputs(resolved, baked, paramsSchema))
	subnets, ok := cluster.Provider.Params["subnets"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, subnets, "my-subnet-a")
	require.Contains(t, subnets, "my-subnet-b")
}

// TestApplyBakedInputs_RejectsUndeclaredKeyDeepInsideMapValueObject is the
// deep-nesting case: an object field nested inside a map's value schema
// still has its own fixed, non-empty field set, so the recursion must
// continue one level further and reject an unknown key at the deepest
// level -- not just at the map value's own top level.
func TestApplyBakedInputs_RejectsUndeclaredKeyDeepInsideMapValueObject(t *testing.T) {
	cluster := &ast.ClusterDoc{Provider: ast.ProviderUse{Use: "yandex"}}
	resolved := &include.Resolved{Cluster: cluster}

	values, err := structpb.NewStruct(map[string]any{
		"provider": map[string]any{
			"vms": map[string]any{
				"my-vm-a": map[string]any{
					"disk": map[string]any{"size_gb": float64(50), "evil_key": "rm -rf /"},
				},
			},
		},
	})
	require.NoError(t, err)
	baked := &spb.Baked{Values: values}

	paramsSchema := spb.NewSchema("ns", "params", "1").
		Fields(
			spb.Map("vms",
				spb.Object("disk", spb.Double("size_gb")).Strict(),
			).Strict(),
		).MustBuild()

	err = schema.ApplyBakedInputs(resolved, baked, paramsSchema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "vms.my-vm-a.disk.evil_key")
	require.Empty(t, cluster.Provider.Params, "no partial application on reject")
}

func TestSplitBakedValues_NilSafe(t *testing.T) {
	wfInputs, providerParams := schema.SplitBakedValues(nil)
	require.Nil(t, wfInputs)
	require.Nil(t, providerParams)
}
