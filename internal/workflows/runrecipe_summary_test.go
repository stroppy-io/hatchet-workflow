package workflows

import (
	"testing"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// postgresHaTestPlan mirrors examples/dsl/postgres-ha's shape (provider
// yandex, a 3-node "db" group + a 1-node "runner" group, etcd/patroni-
// postgres/haproxy sidecars alongside a "stroppy" runner service, and a
// matrix-expanded "bench" job running two workloads against it) — close
// enough to a real compiled recipe to exercise every deriveRunSummary
// heuristic at once.
func postgresHaTestPlan() *dslpb.CompiledPlan {
	return &dslpb.CompiledPlan{
		Provider: &dslpb.ProviderRef{Name: "yandex"},
		MachineGroups: []*dslpb.MachineGroup{
			{Name: "db", Count: 3},
			{Name: "runner", Count: 1},
		},
		Services: []*dslpb.ServiceSpec{
			{Name: "etcd", OnGroup: "db", Image: "quay.io/coreos/etcd:v3.5.14"},
			{Name: "patroni-postgres", OnGroup: "db", Image: "registry.opensource.zalan.do/acid/spilo-16:3.2-p3"},
			{Name: "haproxy", OnGroup: "db", Image: "haproxy:2.9"},
			{Name: "stroppy", OnGroup: "runner", Image: "stroppy:1.2.3"},
		},
		Jobs: []*dslpb.CompiledJob{
			{
				Id:      "bench[workload=insert]",
				OnGroup: "runner",
				Matrix:  map[string]string{"workload": "insert"},
				Action:  &dslpb.CompiledJob_Service{Service: "stroppy"},
			},
			{
				Id:      "bench[workload=select]",
				OnGroup: "runner",
				Matrix:  map[string]string{"workload": "select"},
				Action:  &dslpb.CompiledJob_Service{Service: "stroppy"},
			},
		},
	}
}

func TestDeriveRunSummaryFromCompiledPlan(t *testing.T) {
	summary := deriveRunSummary(postgresHaTestPlan())

	if got, want := summary.GetProvider(), deploymentpb.Provider_PROVIDER_YANDEX; got != want {
		t.Errorf("provider = %s, want %s", got, want)
	}
	if got, want := summary.GetNodeCount(), uint32(4); got != want {
		t.Errorf("node_count = %d, want %d", got, want)
	}
	if got, want := summary.GetTopologyLabel(), "yandex · 4 nodes"; got != want {
		t.Errorf("topology_label = %q, want %q", got, want)
	}
	if got, want := summary.GetDbKind(), domainpb.Database_KIND_POSTGRES; got != want {
		t.Errorf("db_kind = %s, want %s (patroni-postgres service should be recognized as postgres)", got, want)
	}
	if got, want := summary.GetDbVersion(), "3.2-p3"; got != want {
		t.Errorf("db_version = %q, want %q (tag of the matched patroni-postgres image)", got, want)
	}
	if got, want := summary.GetWorkloadName(), "insert+select"; got != want {
		t.Errorf("workload_name = %q, want %q", got, want)
	}
	if got, want := summary.GetStroppyVersion(), "1.2.3"; got != want {
		t.Errorf("stroppy_version = %q, want %q", got, want)
	}
}

// TestDeriveRunSummaryUnrecognizedDbLeavesKindUnspecified asserts the
// heuristic never guesses: a DB service whose name/image matches none of
// dbKindKeywords leaves DbKind at KIND_UNSPECIFIED rather than picking a
// wrong kind — monitoring.dbKindString's own documented "falls back to
// postgres" behavior then applies, which is the pre-existing, acceptable v1
// behavior for a recipe this heuristic cannot classify.
func TestDeriveRunSummaryUnrecognizedDbLeavesKindUnspecified(t *testing.T) {
	plan := &dslpb.CompiledPlan{
		Provider:      &dslpb.ProviderRef{Name: "docker"},
		MachineGroups: []*dslpb.MachineGroup{{Name: "app", Count: 1}},
		Services: []*dslpb.ServiceSpec{
			{Name: "weirddb", OnGroup: "app", Image: "example.com/some-custom-db:9"},
			{Name: "stroppy", OnGroup: "app", Image: "stroppy:latest"},
		},
	}
	summary := deriveRunSummary(plan)
	if got := summary.GetDbKind(); got != domainpb.Database_KIND_UNSPECIFIED {
		t.Errorf("db_kind = %s, want KIND_UNSPECIFIED for an unrecognized service", got)
	}
	if got := summary.GetDbVersion(); got != "" {
		t.Errorf("db_version = %q, want empty (no db service matched, so no service to read a version off)", got)
	}
	if got, want := summary.GetStroppyVersion(), "latest"; got != want {
		t.Errorf("stroppy_version = %q, want %q", got, want)
	}
	// No matrix/with workload and no matching job -> best-effort empty.
	if got := summary.GetWorkloadName(); got != "" {
		t.Errorf("workload_name = %q, want empty (no job references the stroppy service)", got)
	}
}

func TestImageTagIgnoresRegistryPort(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"stroppy:1.2.3", "1.2.3"},
		{"stroppy:latest", "latest"},
		{"registry.example.com:5000/stroppy", ""},
		{"registry.example.com:5000/stroppy:v2", "v2"},
		{"stroppy", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := imageTag(tt.image); got != tt.want {
			t.Errorf("imageTag(%q) = %q, want %q", tt.image, got, tt.want)
		}
	}
}

func TestProviderKindFromName(t *testing.T) {
	tests := []struct {
		name string
		want deploymentpb.Provider
	}{
		{"docker", deploymentpb.Provider_PROVIDER_DOCKER},
		{"yandex", deploymentpb.Provider_PROVIDER_YANDEX},
		{"DOCKER", deploymentpb.Provider_PROVIDER_DOCKER},
		{"nonexistent", deploymentpb.Provider_PROVIDER_UNSPECIFIED},
		{"", deploymentpb.Provider_PROVIDER_UNSPECIFIED},
	}
	for _, tt := range tests {
		if got := providerKindFromName(tt.name); got != tt.want {
			t.Errorf("providerKindFromName(%q) = %s, want %s", tt.name, got, tt.want)
		}
	}
}
