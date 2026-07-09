package lower

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
)

// TestJobInputFields_TypedScalarsNotStringified is the I1-fix regression
// test: a component input declared as `{type: int}` bound to 3 (a Go int,
// per include.checkInputType's "int" case) must lower to a structpb
// NumberValue, not the string "3" (see dslrun.go's documented I1
// limitation, now resolved).
func TestJobInputFields_TypedScalarsNotStringified(t *testing.T) {
	components := []include.BoundComponent{{
		Name:   "ha",
		Doc:    &ast.ComponentDoc{Inputs: map[string]ast.InputSpec{"count": {Type: "int"}}},
		Inputs: map[string]any{"count": 3},
	}}
	resolvedInputs, _, _ := jobInputFields("ha/install", components)

	v, ok := resolvedInputs["count"]
	require.True(t, ok)
	require.Equal(t, float64(3), v.GetNumberValue(), "typed int must round-trip as a structpb NumberValue, not a stringified scalar")
}
