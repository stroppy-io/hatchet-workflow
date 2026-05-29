package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/postgres"
)

func init() { Register("postgres", ExpandPostgres) }

// ExpandPostgres derives the PostgreSQL database VMs from its config:
//   - 1 primary
//   - replica_count replicas
//   - for patroni_ha: a DCS quorum (cluster_size coordinators) + 1 proxy (HAProxy)
//
// Count is correct by construction — no cross-validation needed.
func ExpandPostgres(db map[string]any) []VM {
	vms := []VM{{Role: RoleDatabase, Name: "pg-primary", Shape: ShapeDatabase}}

	for i := 1; i <= mapInt(db, postgres.FieldReplicaCount, 0); i++ {
		vms = append(vms, VM{Role: RoleReplica, Name: fmt.Sprintf("pg-replica-%d", i), Shape: ShapeReplica})
	}

	if mapStr(db, postgres.FieldTopology, postgres.TopologySingle) == postgres.TopologyPatroniHA {
		dcs := mapObj(mapObj(db, postgres.FieldHA), postgres.FieldDCS)
		for i := 1; i <= mapInt(dcs, postgres.FieldClusterSize, 3); i++ {
			vms = append(vms, VM{Role: RoleCoordinator, Name: fmt.Sprintf("etcd-%d", i), Shape: ShapeCoordinator})
		}
		vms = append(vms, VM{Role: RoleProxy, Name: "haproxy-1", Shape: ShapeProxy})
	}

	return vms
}
