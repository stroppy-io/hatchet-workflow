package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/mariadb"
)

func init() { Register("mariadb", ExpandMariaDB) }

// ExpandMariaDB derives the MariaDB database VMs from its config:
//   - 1 primary
//   - replica_count replicas
//   - if ProxySQL is enabled (proxy.use_proxy): 1 proxy
//
// Count is correct by construction — no cross-validation needed.
func ExpandMariaDB(db map[string]any) []VM {
	vms := []VM{{Role: RoleDatabase, Name: "mariadb-primary", Shape: ShapeDatabase}}

	for i := 1; i <= mapInt(db, mariadb.FieldReplicaCount, 0); i++ {
		vms = append(vms, VM{Role: RoleReplica, Name: fmt.Sprintf("mariadb-replica-%d", i), Shape: ShapeDatabase})
	}

	if mapBool(mapObj(db, mariadb.FieldProxy), mariadb.FieldUseProxy, false) {
		vms = append(vms, VM{Role: RoleProxy, Name: "proxysql-1", Shape: ShapeProxy})
	}

	return vms
}
