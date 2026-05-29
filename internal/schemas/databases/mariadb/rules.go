package mariadb

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// dbRules are the DB-internal cross-field rules. They reference only fields of
// THIS schema, so they validate correctly standalone (root = the mariadb form).
// Paths go through rp(pfx, ...) so the schema stays embeddable. Each rule that
// touches a gated subtree leads with a short-circuiting guard.
func dbRules(pfx string) []schemapb.RuleDef {
	var (
		topology = rp(pfx, FieldTopology)
		replicas = rp(pfx, FieldReplicaCount)
	)
	return []schemapb.RuleDef{
		// A Galera cluster needs >= 3 nodes for quorum; with one primary that
		// means at least 2 additional nodes (replica_count >= 2).
		schemapb.Rule(
			fmt.Sprintf("%s || %s >= 2", utils.Ne(topology, TopologyGalera), replicas),
			"galera needs >= 3 nodes for quorum (replica_count >= 2)",
		).ID("mariadb_galera_min_nodes"),
	}
}
