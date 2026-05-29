package ydb

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// dbRules are the DB-internal cross-field rules. They reference only fields of
// THIS schema, so they validate correctly standalone (root = the ydb form).
// Paths go through rp(pfx, ...) so the schema stays embeddable. Each rule that
// touches a gated subtree leads with a short-circuiting guard.
func dbRules(pfx string) []schemapb.RuleDef {
	return []schemapb.RuleDef{
		// mirror-3-dc spreads storage across 3 data centers, so it needs at least
		// 3 storage nodes. The storage object may be absent standalone, so guard
		// against nil before dereferencing its fault_tolerance.
		schemapb.Rule(
			fmt.Sprintf("%s == nil || %s || %s >= 3",
				rp(pfx, FieldStorage),
				utils.Ne(rp(pfx, FieldStorage, FieldFaultTolerance), FaultToleranceMirror3DC),
				rp(pfx, FieldStorageNodeCount)),
			"mirror-3-dc requires at least 3 storage nodes (one per data center)",
		).ID("ydb_mirror3dc_min_nodes"),
	}
}
