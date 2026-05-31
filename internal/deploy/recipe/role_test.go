package recipe

import (
	"sort"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

func TestKindToRole(t *testing.T) {
	cases := map[topology.Component_Kind]expand.Role{
		topology.Component_KIND_DATABASE:    expand.RoleDatabase,
		topology.Component_KIND_REPLICA:     expand.RoleReplica,
		topology.Component_KIND_COORDINATOR: expand.RoleCoordinator,
		topology.Component_KIND_PROXY:       expand.RoleProxy,
		topology.Component_KIND_WORKLOAD:    expand.RoleWorkload,
		topology.Component_KIND_ADDON:       "",
	}
	for kind, want := range cases {
		if got := KindToRole(kind); got != want {
			t.Errorf("KindToRole(%v) = %q, want %q", kind, got, want)
		}
	}
}

func TestInstanceRole(t *testing.T) {
	inst := &topology.Topology_Instance{
		Id:         "n1",
		Components: []*topology.Component{{Kind: topology.Component_KIND_PROXY}},
	}
	if got := InstanceRole(inst); got != expand.RoleProxy {
		t.Fatalf("InstanceRole = %q, want proxy", got)
	}
	// No components -> empty role.
	if got := InstanceRole(&topology.Topology_Instance{Id: "n2"}); got != "" {
		t.Fatalf("InstanceRole(no components) = %q, want empty", got)
	}
}

// TestRoleTierOrder locks the deploy ordering: consensus before DB before proxy
// before workload (so a tier-sorted plan brings dependencies up first).
func TestRoleTierOrder(t *testing.T) {
	roles := []expand.Role{
		expand.RoleWorkload, expand.RoleProxy, expand.RoleDatabase, expand.RoleCoordinator,
	}
	sort.SliceStable(roles, func(i, j int) bool { return roles[i].Tier() < roles[j].Tier() })
	want := []expand.Role{
		expand.RoleCoordinator, expand.RoleDatabase, expand.RoleProxy, expand.RoleWorkload,
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("tier order = %v, want %v", roles, want)
		}
	}
}
