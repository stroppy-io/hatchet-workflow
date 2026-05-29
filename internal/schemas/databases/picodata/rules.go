package picodata

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"
)

// dbRules are the DB-internal cross-field rules. They reference only fields of
// THIS schema, so they validate correctly standalone (root = the picodata form).
// Paths go through rp(pfx, ...) so the schema stays embeddable.
func dbRules(pfx string) []schemapb.RuleDef {
	var (
		instanceCount     = rp(pfx, FieldInstanceCount)
		replicationFactor = rp(pfx, FieldReplicationFactor)
	)
	return []schemapb.RuleDef{
		schemapb.Rule(
			fmt.Sprintf("%s >= %s", instanceCount, replicationFactor),
			"instance_count must be >= replication_factor",
		).ID("picodata_instances_ge_rf"),
	}
}
