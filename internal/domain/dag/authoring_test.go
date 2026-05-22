package dag

import (
	"testing"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

// pgDatabasePreset is a single-host postgres DatabasePreset.
func pgDatabasePreset() *domain.DatabasePreset {
	return &domain.DatabasePreset{
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{{
				Id: "db1", Cores: 4, MemoryGb: 8, DiskGb: 50,
				Components: []*domain.Topology_Component{
					{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE},
				},
			}},
		},
	}
}

// pgWorkloadPreset is a workload that drives the postgres protocol from its own
// load-generator host.
func pgWorkloadPreset() *domain.WorkloadPreset {
	return &domain.WorkloadPreset{
		Workload: &domain.Workload{
			StroppyVersion: "v5.1.3",
			Script:         "tpcc",
			Protocol:       domain.Workload_PROTOCOL_PG,
			Parameters:     &domain.Workload_Parameters{PoolSize: 16, ScaleFactor: 1},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{{
				Id: "load1", Cores: 2, MemoryGb: 4, DiskGb: 20,
				Components: []*domain.Topology_Component{
					{Id: "stroppy", Kind: domain.Topology_Component_KIND_STROPPY},
				},
			}},
			Connections: []*domain.Topology_Connection{
				{From: "stroppy", To: "pg", Kind: domain.Topology_Connection_KIND_FLOW},
			},
		},
	}
}

func diagCodes(diags []*uipb.Diagnostic) map[uipb.DiagnosticCode]uipb.Severity {
	out := map[uipb.DiagnosticCode]uipb.Severity{}
	for _, d := range diags {
		out[d.GetCode()] = d.GetSeverity()
	}
	return out
}

func hasErrorCode(diags []*uipb.Diagnostic, code uipb.DiagnosticCode) bool {
	for _, d := range diags {
		if d.GetCode() == code && d.GetSeverity() == uipb.Severity_SEVERITY_ERROR {
			return true
		}
	}
	return false
}

func TestAssembleFromPresets_Happy(t *testing.T) {
	db := pgDatabasePreset()
	wl := pgWorkloadPreset()

	preset, diags, prov, err := AssembleFromPresets(db, wl, deployment.Provider_PROVIDER_DOCKER)
	if err != nil {
		t.Fatalf("AssembleFromPresets error: %v", err)
	}
	// No blocking ERROR diagnostics for a compatible pair.
	if hasError(diags) {
		for _, d := range diags {
			if d.GetSeverity() == uipb.Severity_SEVERITY_ERROR {
				t.Logf("unexpected error diag: code=%s path=%s msg=%s", d.GetCode(), d.GetFieldPath(), d.GetMessage())
			}
		}
		t.Fatalf("expected no ERROR diagnostics for compatible pg preset")
	}
	// Database + workload carried through.
	if preset.GetDatabase().GetKind() != domain.Database_KIND_POSTGRES {
		t.Errorf("database kind = %s, want POSTGRES", preset.GetDatabase().GetKind())
	}
	if preset.GetWorkload().GetScript() != "tpcc" {
		t.Errorf("workload script = %q, want tpcc", preset.GetWorkload().GetScript())
	}
	// Merged topology: both machines present.
	if got := len(preset.GetTopology().GetMachines()); got != 2 {
		t.Errorf("merged topology machines = %d, want 2", got)
	}
	// Deployment materialized for docker: one spec per machine.
	if preset.GetDeployment().GetProvider() != deployment.Provider_PROVIDER_DOCKER {
		t.Errorf("deployment provider = %s, want DOCKER", preset.GetDeployment().GetProvider())
	}
	if got := len(preset.GetDeployment().GetSpecs()); got != 2 {
		t.Errorf("deployment specs = %d, want 2", got)
	}
	// Provenance: database/workload/deployment sourced correctly.
	if prov.GetFields()["database"] != uipb.ProvenanceSource_PROVENANCE_SOURCE_DATABASE_PRESET {
		t.Errorf("provenance[database] = %s", prov.GetFields()["database"])
	}
	if prov.GetFields()["workload"] != uipb.ProvenanceSource_PROVENANCE_SOURCE_WORKLOAD_PRESET {
		t.Errorf("provenance[workload] = %s", prov.GetFields()["workload"])
	}
	if prov.GetFields()["deployment"] != uipb.ProvenanceSource_PROVENANCE_SOURCE_GENERATED {
		t.Errorf("provenance[deployment] = %s, want GENERATED", prov.GetFields()["deployment"])
	}
}

// TestAssembleFromPresets_ByValue asserts the assemble deep-copies the inputs:
// mutating the source presets afterwards must not change the assembled preset.
func TestAssembleFromPresets_ByValue(t *testing.T) {
	db := pgDatabasePreset()
	wl := pgWorkloadPreset()
	preset, _, _, err := AssembleFromPresets(db, wl, deployment.Provider_PROVIDER_DOCKER)
	if err != nil {
		t.Fatalf("AssembleFromPresets error: %v", err)
	}
	// Mutate the catalog rows after assembly.
	db.GetDatabase().Version = "17"
	wl.GetWorkload().Script = "tpch"
	if preset.GetDatabase().GetVersion() != "16" {
		t.Errorf("assembled db version changed to %q; assemble did not copy by value", preset.GetDatabase().GetVersion())
	}
	if preset.GetWorkload().GetScript() != "tpcc" {
		t.Errorf("assembled workload script changed to %q; assemble did not copy by value", preset.GetWorkload().GetScript())
	}
}

func TestMergeTopology_Union(t *testing.T) {
	dbTopo := pgDatabasePreset().GetTopology()
	wlTopo := pgWorkloadPreset().GetTopology()

	topo, prov, diags := MergeTopology(dbTopo, wlTopo, nil)
	if hasError(diags) {
		t.Fatalf("unexpected ERROR diagnostics merging disjoint topologies: %+v", diags)
	}
	if got := len(topo.GetMachines()); got != 2 {
		t.Fatalf("merged machines = %d, want 2", got)
	}
	if got := len(topo.GetConnections()); got != 1 {
		t.Errorf("merged connections = %d, want 1", got)
	}
	// Provenance per machine source.
	if prov.GetFields()["machines.db1"] != uipb.ProvenanceSource_PROVENANCE_SOURCE_DATABASE_PRESET {
		t.Errorf("provenance[machines.db1] = %s, want DATABASE_PRESET", prov.GetFields()["machines.db1"])
	}
	if prov.GetFields()["machines.load1"] != uipb.ProvenanceSource_PROVENANCE_SOURCE_WORKLOAD_PRESET {
		t.Errorf("provenance[machines.load1] = %s, want WORKLOAD_PRESET", prov.GetFields()["machines.load1"])
	}
}

func TestMergeTopology_Conflict(t *testing.T) {
	// Same machine id "shared" declared with incompatible cores by db and workload.
	dbTopo := &domain.Topology{Machines: []*domain.Topology_Machine{{
		Id: "shared", Cores: 4, MemoryGb: 8,
		Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
	}}}
	wlTopo := &domain.Topology{Machines: []*domain.Topology_Machine{{
		Id: "shared", Cores: 16, MemoryGb: 8,
		Components: []*domain.Topology_Component{{Id: "stroppy", Kind: domain.Topology_Component_KIND_STROPPY}},
	}}}

	topo, _, diags := MergeTopology(dbTopo, wlTopo, nil)
	if !hasErrorCode(diags, uipb.DiagnosticCode_DIAGNOSTIC_CODE_TOPOLOGY_CONFLICT) {
		t.Fatalf("expected TOPOLOGY_CONFLICT error diagnostic, got %+v", diags)
	}
	// Single merged machine; components unioned.
	if got := len(topo.GetMachines()); got != 1 {
		t.Errorf("merged machines = %d, want 1", got)
	}
	if got := len(topo.GetMachines()[0].GetComponents()); got != 2 {
		t.Errorf("merged components on shared machine = %d, want 2", got)
	}
}

func TestMergeTopology_UserPatchOverrides(t *testing.T) {
	dbTopo := pgDatabasePreset().GetTopology()
	wlTopo := pgWorkloadPreset().GetTopology()
	// User bumps db1 to 32 cores.
	patch := &domain.Topology{Machines: []*domain.Topology_Machine{{
		Id: "db1", Cores: 32, MemoryGb: 64, DiskGb: 200,
		Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
	}}}

	topo, prov, _ := MergeTopology(dbTopo, wlTopo, patch)
	var db1 *domain.Topology_Machine
	for _, m := range topo.GetMachines() {
		if m.GetId() == "db1" {
			db1 = m
		}
	}
	if db1 == nil {
		t.Fatalf("db1 missing from merged topology")
	}
	if db1.GetCores() != 32 {
		t.Errorf("db1 cores = %d, want 32 (user patch)", db1.GetCores())
	}
	if prov.GetFields()["machines.db1"] != uipb.ProvenanceSource_PROVENANCE_SOURCE_USER {
		t.Errorf("provenance[machines.db1] = %s, want USER", prov.GetFields()["machines.db1"])
	}
}

func TestCompatibilityDiagnostics_ProtocolUnsupported(t *testing.T) {
	db := &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"}
	// MySQL protocol against a postgres engine is unsupported.
	wl := &domain.Workload{Script: "tpcc", Protocol: domain.Workload_PROTOCOL_MYSQL, StroppyVersion: "v5.1.3"}

	diags := CompatibilityDiagnostics(db, wl)
	if !hasErrorCode(diags, uipb.DiagnosticCode_DIAGNOSTIC_CODE_PROTOCOL_UNSUPPORTED) {
		t.Fatalf("expected PROTOCOL_UNSUPPORTED error, got %+v", diagCodes(diags))
	}
}

func TestCompatibilityDiagnostics_ScriptAndVersion(t *testing.T) {
	db := &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"}
	// Missing script + below-minimum stroppy version.
	wl := &domain.Workload{Protocol: domain.Workload_PROTOCOL_PG, StroppyVersion: "v3.0.0"}

	diags := CompatibilityDiagnostics(db, wl)
	if !hasErrorCode(diags, uipb.DiagnosticCode_DIAGNOSTIC_CODE_SCRIPT_UNSUPPORTED) {
		t.Errorf("expected SCRIPT_UNSUPPORTED error, got %+v", diagCodes(diags))
	}
	if !hasErrorCode(diags, uipb.DiagnosticCode_DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN) {
		t.Errorf("expected STROPPY_VERSION_BELOW_MIN error, got %+v", diagCodes(diags))
	}
}

func TestCompatibilityDiagnostics_CommitVersionBypass(t *testing.T) {
	db := &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"}
	wl := &domain.Workload{Script: "tpcc", Protocol: domain.Workload_PROTOCOL_PG, StroppyVersion: "commit:deadbeef"}

	diags := CompatibilityDiagnostics(db, wl)
	if hasErrorCode(diags, uipb.DiagnosticCode_DIAGNOSTIC_CODE_STROPPY_VERSION_BELOW_MIN) {
		t.Fatalf("commit:<sha> version must bypass the semver floor, got %+v", diagCodes(diags))
	}
}

func TestCheckDeployment_QuotaSums(t *testing.T) {
	preset, _, _, err := AssembleFromPresets(pgDatabasePreset(), pgWorkloadPreset(), deployment.Provider_PROVIDER_YANDEX)
	if err != nil {
		t.Fatalf("AssembleFromPresets error: %v", err)
	}
	quota, feasible, diags := CheckDeployment(preset)
	if !feasible {
		t.Fatalf("expected feasible deployment, diags=%+v", diags)
	}
	got := map[deployment.QuotaResource]uint64{}
	for _, q := range quota {
		got[q.GetResource()] = q.GetRequested()
	}
	// db1: 4 cores/8gb/50disk + load1: 2 cores/4gb/20disk = 6 cores, 12 gb, 70 disk, 2 instances.
	if got[deployment.QuotaResource_QUOTA_RESOURCE_CORES] != 6 {
		t.Errorf("cores quota = %d, want 6", got[deployment.QuotaResource_QUOTA_RESOURCE_CORES])
	}
	if got[deployment.QuotaResource_QUOTA_RESOURCE_MEMORY_GB] != 12 {
		t.Errorf("memory quota = %d, want 12", got[deployment.QuotaResource_QUOTA_RESOURCE_MEMORY_GB])
	}
	if got[deployment.QuotaResource_QUOTA_RESOURCE_SSD_GB] != 70 {
		t.Errorf("ssd quota = %d, want 70", got[deployment.QuotaResource_QUOTA_RESOURCE_SSD_GB])
	}
	if got[deployment.QuotaResource_QUOTA_RESOURCE_INSTANCES] != 2 {
		t.Errorf("instances quota = %d, want 2", got[deployment.QuotaResource_QUOTA_RESOURCE_INSTANCES])
	}
}

func TestCheckDeployment_DockerNoQuota(t *testing.T) {
	preset, _, _, err := AssembleFromPresets(pgDatabasePreset(), pgWorkloadPreset(), deployment.Provider_PROVIDER_DOCKER)
	if err != nil {
		t.Fatalf("AssembleFromPresets error: %v", err)
	}
	quota, feasible, _ := CheckDeployment(preset)
	if !feasible {
		t.Errorf("docker deployment should be feasible")
	}
	if len(quota) != 0 {
		t.Errorf("docker consumes no provider quota, got %d requests", len(quota))
	}
}

func TestProbeWorkload_Static(t *testing.T) {
	preset, _, _, err := AssembleFromPresets(pgDatabasePreset(), pgWorkloadPreset(), deployment.Provider_PROVIDER_DOCKER)
	if err != nil {
		t.Fatalf("AssembleFromPresets error: %v", err)
	}
	version, ok, diags := ProbeWorkload(preset)
	if !ok {
		t.Fatalf("expected probe ok for compatible workload, diags=%+v", diags)
	}
	if version != "v5.1.3" {
		t.Errorf("probe version = %q, want v5.1.3", version)
	}
}

func TestPreviewWorkloadConfig_JSON(t *testing.T) {
	preset, _, _, err := AssembleFromPresets(pgDatabasePreset(), pgWorkloadPreset(), deployment.Provider_PROVIDER_DOCKER)
	if err != nil {
		t.Fatalf("AssembleFromPresets error: %v", err)
	}
	cfg, err := PreviewWorkloadConfig(preset)
	if err != nil {
		t.Fatalf("PreviewWorkloadConfig error: %v", err)
	}
	if cfg == "" {
		t.Fatalf("empty workload config preview")
	}
	// Should mention the postgres URL scheme + the host token.
	if !contains(cfg, "postgresql://") || !contains(cfg, stroppyDBHostToken) {
		t.Errorf("workload config preview missing pg url/token:\n%s", cfg)
	}
	if !contains(cfg, "tpcc") {
		t.Errorf("workload config preview missing script:\n%s", cfg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
