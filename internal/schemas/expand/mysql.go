package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/mysql"
)

func init() { Register("mysql", ExpandMySQL) }

// ExpandMySQL derives the MySQL database VMs from its config:
//   - 1 primary
//   - replica_count replicas
//   - if ProxySQL is enabled (proxy.use_proxy): 1 proxy
//
// Count is correct by construction — no cross-validation needed.
func ExpandMySQL(db map[string]any) []VM {
	vms := []VM{{Role: RoleDatabase, Name: "mysql-primary", Shape: ShapeDatabase}}

	for i := 1; i <= mapInt(db, mysql.FieldReplicaCount, 0); i++ {
		vms = append(vms, VM{Role: RoleReplica, Name: fmt.Sprintf("mysql-replica-%d", i), Shape: ShapeDatabase})
	}

	if mapBool(mapObj(db, mysql.FieldProxy), mysql.FieldUseProxy, false) {
		vms = append(vms, VM{Role: RoleProxy, Name: "proxysql-1", Shape: ShapeProxy})
	}

	return vms
}
