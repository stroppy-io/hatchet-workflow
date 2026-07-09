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

	require.NoError(t, schema.ApplyBakedInputs(resolved, baked))
	require.Equal(t, "ru-central1-a", cluster.Provider.Params["zone"], "existing static param untouched")
	require.Equal(t, float64(5), cluster.Provider.Params["replicas"], "baked provider value overlaid")
}

func TestSplitBakedValues_NilSafe(t *testing.T) {
	wfInputs, providerParams := schema.SplitBakedValues(nil)
	require.Nil(t, wfInputs)
	require.Nil(t, providerParams)
}
