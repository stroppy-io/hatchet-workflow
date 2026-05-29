package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/ydb"
)

func init() { Register("ydb", ExpandYDB) }

// ExpandYDB derives the self-hosted YDB database VMs from its config:
//   - storage_node_count static storage nodes (RoleDatabase, "ydb-storage-i")
//   - for split topology: database_node_count dynamic database/compute nodes
//     (RoleReplica, "ydb-compute-i"). combined co-locates them on storage nodes.
//
// Count is correct by construction — no cross-validation needed.
func ExpandYDB(db map[string]any) []VM {
	var vms []VM

	for i := 1; i <= mapInt(db, ydb.FieldStorageNodeCount, 1); i++ {
		vms = append(vms, VM{Role: RoleDatabase, Name: fmt.Sprintf("ydb-storage-%d", i), Shape: ShapeDatabase})
	}

	if mapStr(db, ydb.FieldTopology, ydb.TopologyCombined) == ydb.TopologySplit {
		for i := 1; i <= mapInt(db, ydb.FieldDatabaseNodeCount, 0); i++ {
			vms = append(vms, VM{Role: RoleReplica, Name: fmt.Sprintf("ydb-compute-%d", i), Shape: ShapeDatabase})
		}
	}

	return vms
}
