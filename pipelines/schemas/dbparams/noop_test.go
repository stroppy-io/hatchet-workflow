package dbparams

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestNoop(t *testing.T) {
	schematest.Run(t, Noop(), schematest.Cases{
		Valid: []map[string]any{{}, {"workers": int64(16)}},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"workers": int64(-1)}, Code: "GTE_VIOLATED", Path: "workers"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
	})
}
