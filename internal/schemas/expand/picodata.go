package expand

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/picodata"
)

func init() { Register("picodata", ExpandPicodata) }

// ExpandPicodata derives the Picodata database VMs from its config. Picodata is a
// shared-nothing cluster of homogeneous peers, so the topology is simply
// instance_count identical RoleDatabase nodes — there is no distinct primary,
// coordinator or proxy role at the logical DB layer.
//
// Count is correct by construction — no cross-validation needed.
func ExpandPicodata(db map[string]any) []VM {
	n := mapInt(db, picodata.FieldInstanceCount, 1)
	if n < 1 {
		n = 1
	}

	vms := make([]VM, 0, n)
	for i := 1; i <= n; i++ {
		vms = append(vms, VM{Role: RoleDatabase, Name: fmt.Sprintf("picodata-%d", i), Shape: ShapeDatabase})
	}

	return vms
}
