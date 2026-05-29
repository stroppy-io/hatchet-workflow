package mysql

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// dbRules are the DB-internal cross-field rules. They reference only fields of
// THIS schema, so they validate correctly standalone (root = the mysql form).
// Paths go through rp(pfx, ...) so the schema stays embeddable. Each rule that
// touches a gated subtree leads with a short-circuiting guard.
func dbRules(pfx string) []schemapb.RuleDef {
	var (
		topology = rp(pfx, FieldTopology)
		replicas = rp(pfx, FieldReplicaCount)
	)
	return []schemapb.RuleDef{
		// InnoDB Group Replication needs an odd quorum of >= 3 members; with one
		// primary that means at least 2 additional members (replica_count >= 2).
		schemapb.Rule(
			fmt.Sprintf("%s || %s >= 2", utils.Ne(topology, TopologyGroupReplication), replicas),
			"group_replication needs an odd quorum of >= 3 members (replica_count >= 2)",
		).ID("mysql_gr_min_nodes"),
	}
}
