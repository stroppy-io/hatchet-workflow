package workload

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// workloadRules are the workload-internal cross-field rules. They reference only
// fields of THIS schema, so they validate correctly standalone (root = the
// workload form). Paths go through rp(pfx, ...) so the schema stays embeddable.
//
// These mirror the FE conditions and the old backend value-range checks
// (validate.go V13–V19) that can't be expressed as a single field constraint:
//   - duration must be non-empty when running in duration mode (the s/m/h suffix
//     itself is enforced by the field Pattern);
//   - iterations must be > 0 to make progress when running in iterations mode.
//
// The pure >=0 / enum range checks (vus, pool_size, scale_factor, k6_mode,
// insert method) are field-level constraints and live in workload.go.
func workloadRules(pfx string) []schemapb.RuleDef {
	var (
		k6mode     = rp(pfx, FieldK6Mode)
		duration   = rp(pfx, FieldDuration)
		iterations = rp(pfx, FieldIterations)
	)
	return []schemapb.RuleDef{
		// In duration mode the duration must be set (Pattern already rejects a
		// malformed suffix; this catches the empty case the gate would skip).
		schemapb.Rule(
			fmt.Sprintf("%s || %s != ''", utils.Ne(k6mode, K6ModeDuration), duration),
			"duration is required (e.g. 60s, 10m, 1h) when k6_mode == duration",
		).ID("duration_required_in_duration_mode"),

		// In iterations mode a zero iteration count makes no progress.
		schemapb.Rule(
			fmt.Sprintf("%s || %s > 0", utils.Ne(k6mode, K6ModeIterations), iterations),
			"iterations must be > 0 when k6_mode == iterations",
		).ID("iterations_positive_in_iterations_mode").Warn(),
	}
}
