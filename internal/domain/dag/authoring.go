package dag

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/render"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

// authoring.go is the Wizard's PURE authoring engine (no persistence, no IO, no
// network). It assembles an immutable TestPreset from a DatabasePreset +
// WorkloadPreset by VALUE, merges their topologies, runs compatibility +
// semantic diagnostics, and produces render/probe previews. The uipb
// (api/ui) Diagnostic / Provenance messages are reused directly (they live in
// that package) — the service layer is a thin trace+delegate wrapper.

// ── diagnostics helpers ──────────────────────────────────────────────────────

func diag(sev uipb.Severity, code uipb.DiagnosticCode, path, msg string) *uipb.Diagnostic {
	return &uipb.Diagnostic{Severity: sev, Code: code, FieldPath: path, Message: msg}
}

func errDiag(code uipb.DiagnosticCode, path, msg string) *uipb.Diagnostic {
	return diag(uipb.Severity_SEVERITY_ERROR, code, path, msg)
}

func warnDiag(code uipb.DiagnosticCode, path, msg string) *uipb.Diagnostic {
	return diag(uipb.Severity_SEVERITY_WARNING, code, path, msg)
}

// ── protocol/script compatibility matrix (DATA, alongside the engine recipes) ─
//
// kindProtocols maps a database kind to the workload wire protocols it speaks.
// This is the recast of the old types.KindProtocols compat data (proto-agnostic,
// could later be a backend table) used to drive PROTOCOL_UNSUPPORTED diagnostics.
var kindProtocols = map[domain.Database_Kind][]domain.Workload_Protocol{
	domain.Database_KIND_POSTGRES:  {domain.Workload_PROTOCOL_PG},
	domain.Database_KIND_MYSQL:     {domain.Workload_PROTOCOL_MYSQL},
	domain.Database_KIND_MARIADB:   {domain.Workload_PROTOCOL_MYSQL},
	domain.Database_KIND_PICODATA:  {domain.Workload_PROTOCOL_PICODATA, domain.Workload_PROTOCOL_PG},
	domain.Database_KIND_YDB:       {domain.Workload_PROTOCOL_YDB_GRPC, domain.Workload_PROTOCOL_YDB_GRPCS},
	domain.Database_KIND_COCKROACH: {domain.Workload_PROTOCOL_COCKROACH, domain.Workload_PROTOCOL_PG},
}

// minStroppyVersion is the lowest stroppy version the backend accepts, read from
// STROPPY_MIN_VERSION (default v5.1.3 — stroppy v5 is the k6-based line the run
// config targets). A "commit:<sha>" pseudo-version bypasses the semver floor (dev
// builds), matching the TestPreset BDD note (B6).
func minStroppyVersion() string {
	if v := os.Getenv("STROPPY_MIN_VERSION"); v != "" {
		return v
	}
	return "v5.1.3"
}

// ── 1. AssembleFromPresets ────────────────────────────────────────────────────

// AssembleFromPresets is Wizard step 1: it takes a DatabasePreset and a
// WorkloadPreset BY VALUE (deep-cloned so editing the catalog later cannot
// mutate the draft), merges them + a materialized deployment into an immutable
// TestPreset, and returns the compatibility diagnostics + field provenance the
// Wizard renders. The provider drives deployment materialization; an
// unknown/empty provider yields a DEPLOYMENT_NOT_MATERIALIZABLE diagnostic
// rather than a hard error (assembly still returns the rest of the preset).
func AssembleFromPresets(
	db *domain.DatabasePreset,
	wl *domain.WorkloadPreset,
	provider deployment.Provider,
) (*domain.TestPreset, []*uipb.Diagnostic, *uipb.Provenance, error) {
	if db == nil || wl == nil {
		return nil, nil, nil, fmt.Errorf("assemble: database and workload presets are required")
	}
	// Copy by value — the draft is independent of the catalog rows.
	dbCopy := proto.Clone(db).(*domain.DatabasePreset)
	wlCopy := proto.Clone(wl).(*domain.WorkloadPreset)

	topo, topoProv, topoDiags := MergeTopology(dbCopy.GetTopology(), wlCopy.GetTopology(), nil)

	preset := &domain.TestPreset{
		Database: dbCopy.GetDatabase(),
		Workload: wlCopy.GetWorkload(),
		Topology: topo,
	}

	prov := &uipb.Provenance{Fields: map[string]uipb.ProvenanceSource{}}
	prov.Fields["database"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_DATABASE_PRESET
	prov.Fields["workload"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_WORKLOAD_PRESET
	mergeProvenanceInto(prov, "topology", topoProv)

	var diags []*uipb.Diagnostic
	diags = append(diags, topoDiags...)
	diags = append(diags, CompatibilityDiagnostics(preset.GetDatabase(), preset.GetWorkload())...)

	// Materialize the deployment from the merged topology (GENERATED).
	intent, err := MaterializeDeploymentIntent(preset, provider)
	if err != nil {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
			"deployment", err.Error()))
	} else {
		preset.Deployment = intent
		prov.Fields["deployment"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_GENERATED
	}
	diags = append(diags, ValidateTestPreset(preset)...)

	return preset, diags, prov, nil
}

// ── 2. AssembleTestPreset (re-assemble after edits) ──────────────────────────

// AssembleTestPreset re-assembles a draft TestPreset after the user edited the
// database / workload / topology (Wizard steps 2-3). It re-runs compatibility +
// semantic diagnostics over the edited preset and re-materializes the deployment
// when the user did NOT pin one (an empty/provider-unset Deployment is treated as
// "regenerate"). Provenance marks the present top-level fields USER (they came in
// on the draft / were edited); a re-generated deployment is GENERATED.
func AssembleTestPreset(preset *domain.TestPreset) (*domain.TestPreset, []*uipb.Diagnostic, *uipb.Provenance, error) {
	if preset == nil {
		return nil, nil, nil, fmt.Errorf("assemble: test preset is required")
	}
	out := proto.Clone(preset).(*domain.TestPreset)

	prov := &uipb.Provenance{Fields: map[string]uipb.ProvenanceSource{}}
	if out.GetDatabase() != nil {
		prov.Fields["database"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_USER
	}
	if out.GetWorkload() != nil {
		prov.Fields["workload"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_USER
	}
	if out.GetTopology() != nil {
		prov.Fields["topology"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_USER
	}

	var diags []*uipb.Diagnostic
	diags = append(diags, CompatibilityDiagnostics(out.GetDatabase(), out.GetWorkload())...)
	diags = append(diags, ValidateTopology(out.GetTopology())...)

	// Re-materialize the deployment unless the user pinned one (provider set +
	// at least one spec). An unpinned deployment regenerates from the topology
	// using the existing intent's provider (or the draft's, if any).
	if !deploymentPinned(out.GetDeployment()) {
		provider := out.GetDeployment().GetProvider()
		intent, err := MaterializeDeploymentIntent(out, provider)
		if err != nil {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
				"deployment", err.Error()))
		} else {
			out.Deployment = intent
			prov.Fields["deployment"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_GENERATED
		}
	} else {
		prov.Fields["deployment"] = uipb.ProvenanceSource_PROVENANCE_SOURCE_USER
	}
	diags = append(diags, ValidateTestPreset(out)...)

	return out, diags, prov, nil
}

// deploymentPinned reports whether the user fixed the deployment intent (a known
// provider with explicit specs) — in which case re-assembly leaves it untouched.
func deploymentPinned(intent *deployment.DeploymentIntent) bool {
	return intent.GetProvider() != deployment.Provider_PROVIDER_UNSPECIFIED && len(intent.GetSpecs()) > 0
}

// ── 4. MergeTopology ──────────────────────────────────────────────────────────

// MergeTopology unions the machines / components / connections of the database
// and workload topologies, then applies userPatch on top (override-by-id). It is
// pure and order-stable. Conflicts (the same machine id declared with
// incompatible cores/memory/disk specs by both inputs) raise
// DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT. Provenance records each machine's source.
func MergeTopology(
	dbTopo, wlTopo, userPatch *domain.Topology,
) (*domain.Topology, *uipb.Provenance, []*uipb.Diagnostic) {
	out := &domain.Topology{}
	prov := &uipb.Provenance{Fields: map[string]uipb.ProvenanceSource{}}
	var diags []*uipb.Diagnostic

	// machines: union by id, db first then workload; detect spec conflicts.
	machineByID := map[string]*domain.Topology_Machine{}
	var machineOrder []string
	addMachines := func(topo *domain.Topology, src uipb.ProvenanceSource) {
		for _, m := range topo.GetMachines() {
			id := m.GetId()
			existing, seen := machineByID[id]
			if !seen {
				machineByID[id] = proto.Clone(m).(*domain.Topology_Machine)
				machineOrder = append(machineOrder, id)
				prov.Fields["machines."+id] = src
				continue
			}
			if !machineSpecsCompatible(existing, m) {
				diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT,
					"machines."+id,
					fmt.Sprintf("machine %q declared with incompatible specs by database and workload topologies", id)))
			}
			// Same id: union components (db wins on the machine spec).
			existing.Components = append(existing.Components, cloneComponents(m.GetComponents())...)
		}
	}
	addMachines(dbTopo, uipb.ProvenanceSource_PROVENANCE_SOURCE_DATABASE_PRESET)
	addMachines(wlTopo, uipb.ProvenanceSource_PROVENANCE_SOURCE_WORKLOAD_PRESET)

	// connections: union both (dedup by from->to->kind), then patch.
	connSeen := map[string]bool{}
	addConns := func(topo *domain.Topology) {
		for _, c := range topo.GetConnections() {
			key := connKey(c)
			if connSeen[key] {
				continue
			}
			connSeen[key] = true
			out.Connections = append(out.Connections, proto.Clone(c).(*domain.Topology_Connection))
		}
	}
	addConns(dbTopo)
	addConns(wlTopo)

	// external_components: union by id (db first).
	extSeen := map[string]bool{}
	addExternal := func(topo *domain.Topology) {
		for _, c := range topo.GetExternalComponents() {
			if extSeen[c.GetId()] {
				continue
			}
			extSeen[c.GetId()] = true
			out.ExternalComponents = append(out.ExternalComponents, proto.Clone(c).(*domain.Topology_Component))
		}
	}
	addExternal(dbTopo)
	addExternal(wlTopo)

	// userPatch: override machines by id (full replace of the patched machine),
	// append unseen connections/externals, mark provenance USER.
	if userPatch != nil {
		for _, m := range userPatch.GetMachines() {
			id := m.GetId()
			if _, seen := machineByID[id]; !seen {
				machineOrder = append(machineOrder, id)
			}
			machineByID[id] = proto.Clone(m).(*domain.Topology_Machine)
			prov.Fields["machines."+id] = uipb.ProvenanceSource_PROVENANCE_SOURCE_USER
		}
		for _, c := range userPatch.GetConnections() {
			key := connKey(c)
			if connSeen[key] {
				continue
			}
			connSeen[key] = true
			out.Connections = append(out.Connections, proto.Clone(c).(*domain.Topology_Connection))
		}
		for _, c := range userPatch.GetExternalComponents() {
			if extSeen[c.GetId()] {
				continue
			}
			extSeen[c.GetId()] = true
			out.ExternalComponents = append(out.ExternalComponents, proto.Clone(c).(*domain.Topology_Component))
		}
		if userPatch.GetTags() != nil {
			out.Tags = proto.Clone(userPatch.GetTags()).(*common.Tags)
		}
	}

	for _, id := range machineOrder {
		out.Machines = append(out.Machines, machineByID[id])
	}
	if out.GetTags() == nil {
		if dbTopo.GetTags() != nil {
			out.Tags = proto.Clone(dbTopo.GetTags()).(*common.Tags)
		} else if wlTopo.GetTags() != nil {
			out.Tags = proto.Clone(wlTopo.GetTags()).(*common.Tags)
		}
	}

	diags = append(diags, ValidateTopology(out)...)
	return out, prov, diags
}

func machineSpecsCompatible(a, b *domain.Topology_Machine) bool {
	// Two declarations of the same machine id are compatible only if the
	// resource intents match (a zero in one side is "unspecified", not a clash).
	eqU32 := func(x, y uint32) bool { return x == 0 || y == 0 || x == y }
	eqU64 := func(x, y uint64) bool { return x == 0 || y == 0 || x == y }
	return eqU32(a.GetCores(), b.GetCores()) &&
		eqU64(a.GetMemoryGb(), b.GetMemoryGb()) &&
		eqU64(a.GetDiskGb(), b.GetDiskGb())
}

func cloneComponents(in []*domain.Topology_Component) []*domain.Topology_Component {
	out := make([]*domain.Topology_Component, 0, len(in))
	for _, c := range in {
		out = append(out, proto.Clone(c).(*domain.Topology_Component))
	}
	return out
}

func connKey(c *domain.Topology_Connection) string {
	return fmt.Sprintf("%s->%s/%d", c.GetFrom(), c.GetTo(), c.GetKind())
}

// mergeProvenanceInto folds a sub-element provenance (e.g. the topology merge's
// per-machine sources) into the parent under a "prefix." namespace.
func mergeProvenanceInto(dst *uipb.Provenance, prefix string, src *uipb.Provenance) {
	if src == nil {
		return
	}
	for k, v := range src.GetFields() {
		dst.Fields[prefix+"."+k] = v
	}
}

// ── compatibility diagnostics ─────────────────────────────────────────────────

// CompatibilityDiagnostics is the cross-entity compatibility check where
// Database.Kind meets Workload.Protocol/Script/StroppyVersion (the old
// run.ValidateConfig, recast onto the engine recipe matrix + protocol table). It
// emits PROTOCOL_UNSUPPORTED (kind cannot speak the workload protocol),
// SCRIPT_UNSUPPORTED (no script declared), and STROPPY_VERSION_BELOW_MIN.
func CompatibilityDiagnostics(db *domain.Database, wl *domain.Workload) []*uipb.Diagnostic {
	var diags []*uipb.Diagnostic
	if db == nil || wl == nil {
		return diags
	}

	// The (kind, version) cell must exist in the engine recipe matrix.
	if engine := kindString(db.GetKind()); engine != "" {
		if _, ok := EngineRecipe(engine, db.GetVersion()); !ok {
			diags = append(diags, warnDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_PROTOCOL_UNSUPPORTED,
				"database.version",
				fmt.Sprintf("engine %s/%s is not in the supported matrix (default %s)",
					engine, db.GetVersion(), DefaultVersion(engine))))
		}
	}

	// Protocol must be one the db kind speaks.
	if !protocolSupported(db.GetKind(), wl.GetProtocol()) {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_PROTOCOL_UNSUPPORTED,
			"workload.protocol",
			fmt.Sprintf("database kind %s does not support workload protocol %s",
				db.GetKind(), wl.GetProtocol())))
	}

	// A workload must declare a script (the load profile to run).
	if wl.GetScript() == "" {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_SCRIPT_UNSUPPORTED,
			"workload.script", "workload script is required"))
	}

	// stroppy version floor (commit:<sha> bypasses the semver floor).
	if v := wl.GetStroppyVersion(); v != "" && !isCommitVersion(v) && versionBelow(v, minStroppyVersion()) {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN,
			"workload.stroppy_version",
			fmt.Sprintf("stroppy version %s is below the required minimum %s", v, minStroppyVersion())))
	}

	return diags
}

func protocolSupported(kind domain.Database_Kind, proto domain.Workload_Protocol) bool {
	if proto == domain.Workload_PROTOCOL_UNSPECIFIED {
		return false
	}
	for _, p := range kindProtocols[kind] {
		if p == proto {
			return true
		}
	}
	return false
}

// isCommitVersion reports a "commit:<sha>" dev pseudo-version (bypasses semver).
func isCommitVersion(v string) bool {
	return len(v) > 7 && v[:7] == "commit:"
}

// versionBelow does a lenient numeric semver compare (vMAJOR.MINOR.PATCH). A
// version it cannot parse is treated as NOT below (don't block on unknowns).
func versionBelow(v, min string) bool {
	va, oka := parseSemver(v)
	vb, okb := parseSemver(min)
	if !oka || !okb {
		return false
	}
	for i := 0; i < 3; i++ {
		if va[i] != vb[i] {
			return va[i] < vb[i]
		}
	}
	return false
}

func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	part := 0
	cur := 0
	has := false
	for i := 0; i < len(v); i++ {
		ch := v[i]
		if ch >= '0' && ch <= '9' {
			cur = cur*10 + int(ch-'0')
			has = true
			continue
		}
		if ch == '.' {
			if part > 2 {
				break
			}
			out[part] = cur
			part++
			cur = 0
			continue
		}
		// stop at any pre-release/build suffix (e.g. -rc1, +meta).
		break
	}
	if !has {
		return out, false
	}
	if part <= 2 {
		out[part] = cur
	}
	return out, true
}

// ── semantic validation (beyond proto validate.rules) ─────────────────────────

// ValidateTestPreset runs the semantic checks on a fully-assembled preset: the
// presence of database/workload/topology, cross-entity compatibility, topology
// shape, and (when present) deployment materializability against the topology.
func ValidateTestPreset(preset *domain.TestPreset) []*uipb.Diagnostic {
	var diags []*uipb.Diagnostic
	if preset == nil {
		return []*uipb.Diagnostic{errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_UNSPECIFIED, "", "test preset is nil")}
	}
	if preset.GetDatabase() == nil {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_UNSPECIFIED, "database", "database is required"))
	}
	if preset.GetWorkload() == nil {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_UNSPECIFIED, "workload", "workload is required"))
	}
	diags = append(diags, CompatibilityDiagnostics(preset.GetDatabase(), preset.GetWorkload())...)
	diags = append(diags, ValidateTopology(preset.GetTopology())...)
	if preset.GetDeployment() != nil {
		diags = append(diags, validateDeploymentCoversTopology(preset.GetDeployment(), preset.GetTopology())...)
	}
	return diags
}

// ValidateTopology runs the semantic topology checks: at least one machine, at
// least one DATABASE component, unique machine + component ids, and connection
// endpoints that resolve to a known component (machine or external).
func ValidateTopology(topo *domain.Topology) []*uipb.Diagnostic {
	var diags []*uipb.Diagnostic
	if topo == nil {
		return []*uipb.Diagnostic{errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT, "topology", "topology is required")}
	}
	if len(topo.GetMachines()) == 0 {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT, "topology.machines", "topology has no machines"))
	}

	machineIDs := map[string]bool{}
	componentIDs := map[string]bool{}
	hasDatabase := false
	for _, m := range topo.GetMachines() {
		if machineIDs[m.GetId()] {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT,
				"machines."+m.GetId(), fmt.Sprintf("duplicate machine id %q", m.GetId())))
		}
		machineIDs[m.GetId()] = true
		for _, c := range m.GetComponents() {
			if componentIDs[c.GetId()] {
				diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT,
					"components."+c.GetId(), fmt.Sprintf("duplicate component id %q", c.GetId())))
			}
			componentIDs[c.GetId()] = true
			if c.GetKind() == domain.Topology_Component_KIND_DATABASE {
				hasDatabase = true
			}
		}
	}
	for _, c := range topo.GetExternalComponents() {
		componentIDs[c.GetId()] = true
		if c.GetKind() == domain.Topology_Component_KIND_DATABASE {
			hasDatabase = true
		}
	}
	if !hasDatabase && len(topo.GetMachines()) > 0 {
		diags = append(diags, warnDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT,
			"topology", "topology has no DATABASE component"))
	}
	for _, conn := range topo.GetConnections() {
		if conn.GetFrom() != "" && !componentIDs[conn.GetFrom()] {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT,
				"connections", fmt.Sprintf("connection source %q is not a known component", conn.GetFrom())))
		}
		if conn.GetTo() != "" && !componentIDs[conn.GetTo()] {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT,
				"connections", fmt.Sprintf("connection target %q is not a known component", conn.GetTo())))
		}
	}
	return diags
}

// ValidateDeploymentIntent checks a materialized deployment is self-consistent: a
// known provider and one spec per id, each spec carrying the right provider body.
func ValidateDeploymentIntent(intent *deployment.DeploymentIntent) []*uipb.Diagnostic {
	var diags []*uipb.Diagnostic
	if intent == nil {
		return []*uipb.Diagnostic{errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE, "deployment", "deployment intent is nil")}
	}
	if intent.GetProvider() == deployment.Provider_PROVIDER_UNSPECIFIED {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
			"deployment.provider", "deployment provider is unspecified"))
	}
	if len(intent.GetSpecs()) == 0 {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
			"deployment.specs", "deployment has no specs"))
	}
	specIDs := map[string]bool{}
	for _, s := range intent.GetSpecs() {
		if specIDs[s.GetId()] {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
				"deployment.specs", fmt.Sprintf("duplicate spec id %q", s.GetId())))
		}
		specIDs[s.GetId()] = true
		if s.GetSpec() == nil {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
				"deployment.specs."+s.GetId(), fmt.Sprintf("spec %q carries no provider body", s.GetId())))
		}
	}
	return diags
}

// validateDeploymentCoversTopology asserts every topology machine has a matching
// deployment spec — otherwise the deployment cannot be materialized for that host.
func validateDeploymentCoversTopology(intent *deployment.DeploymentIntent, topo *domain.Topology) []*uipb.Diagnostic {
	var diags []*uipb.Diagnostic
	specIDs := map[string]bool{}
	for _, s := range intent.GetSpecs() {
		specIDs[s.GetId()] = true
	}
	for _, m := range topo.GetMachines() {
		if !specIDs[m.GetId()] {
			diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
				"deployment.specs",
				fmt.Sprintf("machine %q has no deployment spec", m.GetId())))
		}
	}
	return diags
}

// ── 5. CheckDeployment (quota feasibility) ────────────────────────────────────

// CheckDeployment derives the provider quota requests from the topology (the sum
// of cores, memory, disk and instance counts) and reports a best-effort
// feasibility. It is PURE: it sums the request only — the real provider quota
// check (comparing against the account's remaining quota) stays in
// services/yandexcloud. feasible is false only when the preset itself is
// un-deployable (no materialized deployment / missing specs).
func CheckDeployment(preset *domain.TestPreset) ([]*deployment.QuotaRequest, bool, []*uipb.Diagnostic) {
	var diags []*uipb.Diagnostic
	if preset == nil {
		return nil, false, []*uipb.Diagnostic{errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE, "", "test preset is nil")}
	}
	provider := preset.GetDeployment().GetProvider()
	quota := QuotaForTopology(preset.GetTopology(), provider)

	diags = append(diags, ValidateTopology(preset.GetTopology())...)
	if preset.GetDeployment() == nil {
		diags = append(diags, errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_DEPLOYMENT_NOT_MATERIALIZABLE,
			"deployment", "deployment is not materialized"))
	} else {
		diags = append(diags, ValidateDeploymentIntent(preset.GetDeployment())...)
		diags = append(diags, validateDeploymentCoversTopology(preset.GetDeployment(), preset.GetTopology())...)
	}

	feasible := !hasError(diags)
	return quota, feasible, diags
}

// QuotaForTopology sums the topology's resource intents into provider
// QuotaRequests (cores, memory GB, SSD GB, instance count). Disk is summed as
// boot disk + every data disk and reported as SSD (the deployment layer picks
// the concrete disk type). Docker provisioning consumes no provider quota, so it
// yields an empty request set.
func QuotaForTopology(topo *domain.Topology, provider deployment.Provider) []*deployment.QuotaRequest {
	if provider == deployment.Provider_PROVIDER_DOCKER {
		return nil
	}
	var cores, memGB, diskGB, instances uint64
	for _, m := range topo.GetMachines() {
		cores += uint64(m.GetCores())
		memGB += m.GetMemoryGb()
		diskGB += m.GetDiskGb()
		for _, d := range m.GetDataDisksGb() {
			diskGB += d
		}
		instances++
	}
	if instances == 0 {
		return nil
	}
	mk := func(res deployment.QuotaResource, n uint64) *deployment.QuotaRequest {
		return &deployment.QuotaRequest{Provider: provider, Resource: res, Requested: n}
	}
	return []*deployment.QuotaRequest{
		mk(deployment.QuotaResource_QUOTA_RESOURCE_CORES, cores),
		mk(deployment.QuotaResource_QUOTA_RESOURCE_MEMORY_GB, memGB),
		mk(deployment.QuotaResource_QUOTA_RESOURCE_SSD_GB, diskGB),
		mk(deployment.QuotaResource_QUOTA_RESOURCE_INSTANCES, instances),
	}
}

// hasError reports whether any diagnostic is SEVERITY_ERROR (a submit blocker).
func hasError(diags []*uipb.Diagnostic) bool {
	for _, d := range diags {
		if d.GetSeverity() == uipb.Severity_SEVERITY_ERROR {
			return true
		}
	}
	return false
}

// ── 2/3. render + probe previews ──────────────────────────────────────────────

// PreviewDatabaseRender renders the on-host config files for every DATABASE /
// COORDINATOR / PROXY / MONITOR component of the preset's topology (preview ==
// execution, B3 — it reuses the same render layer the agent writes). Components
// with no file config (ydb/cockroach cluster engines) contribute nothing.
func PreviewDatabaseRender(preset *domain.TestPreset) ([]*renderpb.Config, error) {
	if preset == nil {
		return nil, fmt.Errorf("preview: test preset is required")
	}
	db := preset.GetDatabase()
	topo := preset.GetTopology()
	view := viewTopology(topo)
	out := make([]*renderpb.Config, 0)
	for _, m := range topo.GetMachines() {
		memMB := int(m.GetMemoryGb()) * 1024
		for _, c := range m.GetComponents() {
			switch c.GetKind() {
			case domain.Topology_Component_KIND_DATABASE,
				domain.Topology_Component_KIND_COORDINATOR,
				domain.Topology_Component_KIND_PROXY,
				domain.Topology_Component_KIND_MONITOR:
				cfg, err := componentConfig(c, db, topo, memMB)
				if err != nil {
					return nil, fmt.Errorf("preview render %q: %w", c.GetId(), err)
				}
				if cfg != nil && len(cfg.GetItems()) > 0 {
					out = append(out, cfg)
				}
			}
		}
	}
	// Fallback: a topology with no machine-hosted DATABASE still previews the
	// single-host database config so the Wizard shows something for step 2.
	if len(out) == 0 && db != nil {
		if cfg, err := render.RenderDatabase(db, defaultMemoryMB(view)); err == nil && len(cfg.GetItems()) > 0 {
			out = append(out, cfg)
		}
	}
	return out, nil
}

func defaultMemoryMB(v topologyView) int {
	if v.dbMemoryMB > 0 {
		return v.dbMemoryMB
	}
	return 4096 // sensible single-host default for a preview
}

// PreviewWorkloadConfig produces a best-effort preview of the stroppy run config
// (JSON) the agent would execute for the workload: the connection URL (engine
// scheme + late-binding host token), script, protocol, stroppy version and the
// run parameters. The cloud does not currently assemble a full stroppy-v4 config
// document — the agent builds it from these fields — so this is a faithful
// projection of the inputs, not the byte-identical artifact.
//
// TODO(authoring): once the stroppy-v4 run-config schema is wired into a render
// helper, swap this hand-built JSON for the real generated document so
// preview == execution holds for the workload config too.
func PreviewWorkloadConfig(preset *domain.TestPreset) (string, error) {
	if preset == nil || preset.GetWorkload() == nil {
		return "", fmt.Errorf("preview: workload is required")
	}
	wl := preset.GetWorkload()
	db := preset.GetDatabase()
	url := ""
	if db != nil {
		url = stroppyURL(db)
	}
	p := wl.GetParameters()
	doc := map[string]any{
		"url":             url,
		"protocol":        protocolName(wl.GetProtocol()),
		"script":          wl.GetScript(),
		"stroppy_version": wl.GetStroppyVersion(),
		"pool_size":       p.GetPoolSize(),
		"scale_factor":    p.GetScaleFactor(),
	}
	if len(p.GetSteps()) > 0 {
		doc["steps"] = p.GetSteps()
	}
	if len(p.GetNoSteps()) > 0 {
		doc["no_steps"] = p.GetNoSteps()
	}
	if env := p.GetEnv(); len(env) > 0 {
		// emit env as a sorted slice of "k=v" for deterministic preview output.
		pairs := make([]string, 0, len(env))
		for _, k := range envKeys(env) {
			pairs = append(pairs, k+"="+env[k])
		}
		doc["env"] = pairs
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("preview: marshal workload config: %w", err)
	}
	return string(b), nil
}

// ProbeWorkload is the PURE workload probe: it checks the workload's declared
// stroppy version / protocol / script against the compatibility matrix WITHOUT
// any network or agent call. ok is true only when no blocking (ERROR)
// diagnostic is raised; stroppy_version echoes the declared version.
//
// TODO(authoring): a live agent probe (actually invoking `stroppy --version`
// against the resolved binary) can replace this static check once the
// agent-probe RPC exists; the signature already returns the resolved version.
func ProbeWorkload(preset *domain.TestPreset) (string, bool, []*uipb.Diagnostic) {
	if preset == nil || preset.GetWorkload() == nil {
		return "", false, []*uipb.Diagnostic{errDiag(uipb.DiagnosticCode_DIAGNOSTIC_CODE_SCRIPT_UNSUPPORTED, "workload", "workload is required")}
	}
	diags := CompatibilityDiagnostics(preset.GetDatabase(), preset.GetWorkload())
	ok := !hasError(diags)
	return preset.GetWorkload().GetStroppyVersion(), ok, diags
}

func protocolName(p domain.Workload_Protocol) string {
	if name, ok := domain.Workload_Protocol_name[int32(p)]; ok {
		return name
	}
	return p.String()
}

func envKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
