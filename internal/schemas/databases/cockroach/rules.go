package cockroach

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// dbRules are the DB-internal cross-field rules. They reference only fields of
// THIS schema, so they validate correctly standalone (root = the cockroach
// form). Paths go through rp(pfx, ...) so the schema stays embeddable. Each rule
// that touches the gated cluster subtree leads with a short-circuiting guard.
func dbRules(pfx string) []schemapb.RuleDef {
	var (
		topology  = rp(pfx, FieldTopology)
		nodeCount = rp(pfx, FieldNodeCount)
		rf        = rp(pfx, FieldCluster, FieldReplicationFactor)
	)
	return []schemapb.RuleDef{
		schemapb.Rule(
			fmt.Sprintf("%s || %s >= 3", utils.Ne(topology, TopologyCluster), nodeCount),
			"topology=cluster requires node_count >= 3",
		).ID("cockroach_cluster_min_nodes"),

		schemapb.Rule(
			fmt.Sprintf("%s || %s <= %s", utils.Ne(topology, TopologyCluster), rf, nodeCount),
			"replication_factor must be <= node_count",
		).ID("cockroach_rf_le_nodes"),
	}
}
