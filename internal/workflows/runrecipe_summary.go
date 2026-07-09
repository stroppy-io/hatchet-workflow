package workflows

import (
	"fmt"
	"sort"
	"strings"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// deriveRunSummary computes the models.Run.Summary facets a recipe run's
// CompiledPlan carries — Provider/NodeCount/TopologyLabel/DbKind/
// WorkloadName/StroppyVersion — right after RunRecipeWorkflow's compile
// stage succeeds (see run's persistRunSummary call in runrecipe.go). Every
// kept feature that reads a recipe run (rating boards, the metrics tab's
// dbKindString, compare, share, the runs table) reads Summary, never a
// baked domain.TestRun spec (Run has none) — so this is the one place a
// recipe run's queryable facets get filled in.
//
// Every heuristic below is best-effort and documented at its own helper; an
// unrecognized/absent value is left at its Go zero value rather than
// guessed, and none of this ever fails the workflow.
func deriveRunSummary(plan *dslpb.CompiledPlan) *models.Run_Summary {
	groups := plan.GetMachineGroups()
	services := plan.GetServices()
	stroppy := findStroppySvc(services)
	dbKind, dbSvc := inferDbKind(services, stroppy)

	return &models.Run_Summary{
		Provider:       providerKindFromName(plan.GetProvider().GetName()),
		NodeCount:      nodeCount(groups),
		TopologyLabel:  topologyLabel(plan.GetProvider().GetName(), groups),
		DbKind:         dbKind,
		DbVersion:      imageTag(dbSvc.GetImage()),
		WorkloadName:   workloadName(plan.GetJobs(), stroppy),
		StroppyVersion: imageTag(stroppy.GetImage()),
	}
}

// providerKindFromName maps a dslpb.ProviderRef.Name (the recipe author's
// cluster.yaml `provider.use` string, e.g. "docker"/"yandex") onto the
// deployment.Provider enum via the enum's own generated string table
// (Provider_value["PROVIDER_"+upper(name)]), so a new provider only needs
// adding to the proto enum, never a parallel switch here. An unknown/empty
// name yields PROVIDER_UNSPECIFIED.
func providerKindFromName(name string) deploymentpb.Provider {
	if name == "" {
		return deploymentpb.Provider_PROVIDER_UNSPECIFIED
	}
	if v, ok := deploymentpb.Provider_value["PROVIDER_"+strings.ToUpper(name)]; ok {
		return deploymentpb.Provider(v)
	}
	return deploymentpb.Provider_PROVIDER_UNSPECIFIED
}

// nodeCount sums every machine_groups entry's count — the total node count
// of the topology the plan requests.
func nodeCount(groups []*dslpb.MachineGroup) uint32 {
	var total uint32
	for _, g := range groups {
		total += g.GetCount()
	}
	return total
}

// topologyLabel is the runs table's human-readable topology summary, e.g.
// "yandex · 4 nodes". Falls back to just the node count when the provider
// name is empty (compile failed to resolve one — should not normally reach
// here since compile succeeded, but defensive all the same).
func topologyLabel(providerName string, groups []*dslpb.MachineGroup) string {
	n := nodeCount(groups)
	if providerName == "" {
		return fmt.Sprintf("%d nodes", n)
	}
	return fmt.Sprintf("%s · %d nodes", providerName, n)
}

// findStroppySvc identifies the recipe's benchmark-runner ServiceSpec: by
// convention (see examples/dsl/postgres-ha) it is named "stroppy" or its
// image is the "stroppy" image — the compiler enforces neither, but every
// recipe needs exactly one runner service to have a workload at all, so this
// best-effort match is the anchor both DbKind (§ "the primary DB service is
// whichever ServiceSpec is NOT this one") and WorkloadName/StroppyVersion
// key off. Returns nil (all its accessors are nil-safe: GetName/GetImage
// return "") if no service matches, e.g. a hand-rolled recipe that named its
// runner service something else — StroppyVersion/WorkloadName are then left
// blank rather than guessed.
func findStroppySvc(services []*dslpb.ServiceSpec) *dslpb.ServiceSpec {
	for _, svc := range services {
		name := strings.ToLower(svc.GetName())
		image := strings.ToLower(svc.GetImage())
		if name == "stroppy" || strings.HasPrefix(image, "stroppy:") || strings.HasPrefix(image, "stroppy/") {
			return svc
		}
	}
	return nil
}

// dbKindKeywords maps a case-insensitive substring found in a ServiceSpec's
// name or image to the domain.Database.Kind it implies, checked in this
// order (most specific first — e.g. "oriole" before "postgres", since
// OrioleDB is itself a patched Postgres and its image/service name may well
// contain "postgres" too). Mirrors the exact Kind set monitoring.go's
// dbKindString already knows how to route to a metrics query.
var dbKindKeywords = []struct { //nolint:gochecknoglobals // static lookup table, read-only.
	keyword string
	kind    domainpb.Database_Kind
}{
	{"oriole", domainpb.Database_KIND_ORIOLEDB},
	{"cockroach", domainpb.Database_KIND_COCKROACH},
	{"mariadb", domainpb.Database_KIND_MARIADB},
	{"mysql", domainpb.Database_KIND_MYSQL},
	{"ydb", domainpb.Database_KIND_YDB},
	{"picodata", domainpb.Database_KIND_PICODATA},
	{"patroni", domainpb.Database_KIND_POSTGRES},
	{"spilo", domainpb.Database_KIND_POSTGRES},
	{"postgres", domainpb.Database_KIND_POSTGRES},
}

// inferDbKind heuristically finds the recipe's primary database engine: the
// first ServiceSpec, in plan declaration order, that (a) is not the stroppy
// runner service and (b) has a name or image matching one of
// dbKindKeywords. This intentionally does NOT require a recipe author to
// label anything — sidecar/infra services with no DB keyword match (e.g.
// "etcd", "haproxy") are silently skipped, exactly like the DB service
// itself would be if it were named unrecognizably. Returns
// (KIND_UNSPECIFIED, nil) when nothing matches; monitoring.dbKindString
// already falls back to "postgres" for that case (documented, pre-existing
// behavior — see its own doc comment), so an unrecognized recipe's Metrics
// tab still renders, just against the wrong PromQL until the recipe adopts
// a recognized service name/image. The matched ServiceSpec is also
// returned (nil when nothing matches) so deriveRunSummary can read
// DbVersion off the SAME service DbKind came from, via the same
// imageTag helper StroppyVersion already uses for the runner service —
// there is deliberately no separate "db version" heuristic.
func inferDbKind(services []*dslpb.ServiceSpec, stroppy *dslpb.ServiceSpec) (domainpb.Database_Kind, *dslpb.ServiceSpec) {
	for _, svc := range services {
		if svc == stroppy {
			continue
		}
		haystack := strings.ToLower(svc.GetName()) + " " + strings.ToLower(svc.GetImage())
		for _, kw := range dbKindKeywords {
			if strings.Contains(haystack, kw.keyword) {
				return kw.kind, svc
			}
		}
	}
	return domainpb.Database_KIND_UNSPECIFIED, nil
}

// workloadName is the stroppy service's workload(s) — read from the
// matrix-expanded CompiledJob entries that run the stroppy service (job.
// GetService() == stroppy's name; see dsl compiler's matrix expansion,
// e.g. workflow.yaml's `matrix: {workload: [insert, select]}` produces
// jobs "bench[workload=insert]"/"bench[workload=select]", each carrying
// Matrix{"workload": "insert"}/{"workload": "select"}). Falls back to the
// job's `with["workload"]` for a recipe that passes it as a plain arg
// instead of a matrix axis. Every distinct value found (across every
// matching job) is joined with "+" (e.g. "insert+select") since a
// multi-workload recipe run does not have one single workload — this is
// best-effort display, not a queryable/exact-match facet. Empty if the
// stroppy service could not be identified or no job references it.
func workloadName(jobs []*dslpb.CompiledJob, stroppy *dslpb.ServiceSpec) string {
	if stroppy == nil {
		return ""
	}
	seen := map[string]struct{}{}
	for _, job := range jobs {
		if job.GetService() != stroppy.GetName() {
			continue
		}
		w := job.GetMatrix()["workload"]
		if w == "" {
			w = job.GetWith()["workload"]
		}
		if w == "" {
			continue
		}
		seen[w] = struct{}{}
	}
	if len(seen) == 0 {
		return ""
	}
	values := make([]string, 0, len(seen))
	for w := range seen {
		values = append(values, w)
	}
	sort.Strings(values)
	return strings.Join(values, "+")
}

// imageTag extracts the tag portion of a Docker image reference (e.g.
// "stroppy:1.2.3" -> "1.2.3"), used as StroppyVersion. Correctly ignores a
// registry host's own ":port" (e.g. "registry.example.com:5000/stroppy" has
// no tag) by only looking for ":" after the last "/" — the image-name
// segment. Returns "" for an untagged reference or an empty image.
func imageTag(image string) string {
	if image == "" {
		return ""
	}
	name := image
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		name = image[idx+1:]
	}
	idx := strings.LastIndex(name, ":")
	if idx < 0 {
		return ""
	}
	return name[idx+1:]
}
