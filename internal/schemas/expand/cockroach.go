package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/cockroach"
)

func init() { Register("cockroach", ExpandCockroach) }

// ExpandCockroach derives the CockroachDB database VMs from its config.
// CockroachDB is homogeneous and symmetric (no master/replica roles), so the
// topology is simply node_count identical RoleDatabase nodes.
//
// Count is correct by construction — no cross-validation needed.
func ExpandCockroach(db map[string]any) []VM {
	var vms []VM
	for i := 1; i <= mapInt(db, cockroach.FieldNodeCount, 1); i++ {
		vms = append(vms, VM{Role: RoleDatabase, Name: fmt.Sprintf("crdb-%d", i), Shape: ShapeDatabase})
	}
	return vms
}
