package lower_test

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/lower"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

func plainCluster() *ast.ClusterDoc {
	return &ast.ClusterDoc{
		Provider: ast.ProviderUse{Use: "docker", Params: map[string]any{"zone": "a"}},
		Machines: map[string]ast.MachineGroup{},
		Services: map[string]ast.Service{},
	}
}

func plainDomain(cluster *ast.ClusterDoc) *graph.Domain {
	provider := &ast.ProviderManifest{Name: cluster.Provider.Use}
	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		panic(diags)
	}
	return dom
}

// TestLowerWriteFileStep locks the WriteFile -> write_file mapping: Dest maps
// onto common.File_Info.Path, Content onto common.File_Text.Text.
func TestLowerWriteFileStep(t *testing.T) {
	jobs := map[string]ast.Job{
		"cfg": {Steps: []ast.Step{
			{WriteFile: &ast.WriteFile{Dest: "/etc/app/config.yaml", Content: "key: ${{ inputs.x }}"}},
		}},
	}
	cluster := plainCluster()
	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	agent := plan.GetJobs()[0].GetSteps().GetSteps()[0].GetAgent()
	wf := agent.GetWriteFile()
	if wf == nil {
		t.Fatal("expected write_file to be set")
	}
	if got := wf.GetInfo().GetPath(); got != "/etc/app/config.yaml" {
		t.Fatalf("write_file path = %q", got)
	}
	if got := wf.GetText(); got != "key: ${{ inputs.x }}" {
		t.Fatalf("write_file text = %q, want the ${{ }} expression kept unevaluated", got)
	}
	if got := agent.GetId(); got != "cfg/0" {
		t.Fatalf("agent id = %q, want %q", got, "cfg/0")
	}
}

// TestLowerFetchStep locks the Fetch -> fetch_file mapping: URL maps onto
// common.File_AsRef.Uri, Dest onto common.File_Info.Path.
func TestLowerFetchStep(t *testing.T) {
	jobs := map[string]ast.Job{
		"dl": {Steps: []ast.Step{
			{Fetch: &ast.Fetch{URL: "https://example.com/bin", Dest: "/usr/local/bin/tool"}},
		}},
	}
	cluster := plainCluster()
	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	agent := plan.GetJobs()[0].GetSteps().GetSteps()[0].GetAgent()
	ff := agent.GetFetchFile()
	if ff == nil {
		t.Fatal("expected fetch_file to be set")
	}
	if got := ff.GetInfo().GetPath(); got != "/usr/local/bin/tool" {
		t.Fatalf("fetch_file path = %q", got)
	}
	asRef, ok := ff.GetContent().(*commonpb.File_AsRef_)
	if !ok || asRef.AsRef.GetUri() != "https://example.com/bin" {
		t.Fatalf("fetch_file content = %+v, want AsRef.Uri = %q", ff.GetContent(), "https://example.com/bin")
	}
}

// TestLowerDirStep locks the Dir -> create_dir mapping.
func TestLowerDirStep(t *testing.T) {
	jobs := map[string]ast.Job{
		"mk": {Steps: []ast.Step{{Dir: "/var/lib/app"}}},
	}
	cluster := plainCluster()
	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	agent := plan.GetJobs()[0].GetSteps().GetSteps()[0].GetAgent()
	dir := agent.GetCreateDir()
	if dir == nil {
		t.Fatal("expected create_dir to be set")
	}
	if got := dir.GetInfo().GetPath(); got != "/var/lib/app" {
		t.Fatalf("create_dir path = %q", got)
	}
	if !dir.GetCreateParents() {
		t.Fatal("expected create_parents to be true")
	}
}

// TestLowerStepWithNoActionErrors locks that a Step reaching Lower with none
// of Cmd/WriteFile/Fetch/Dir/Wait set (a shape ast.DecodeWorkflow itself
// already rejects — see decodeStep's "expected exactly one action"
// diagnostic — so this can only happen via a caller bypassing that decode
// step, or a bug in an earlier compiler stage) is reported as an error
// rather than a panic or a silently-empty DslStep.
func TestLowerStepWithNoActionErrors(t *testing.T) {
	jobs := map[string]ast.Job{
		"bad": {Steps: []ast.Step{{}}},
	}
	cluster := plainCluster()

	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Lower panicked: %v", r)
			}
		}()
		_, err = lower.Lower(cluster, plainDomain(cluster), jobs)
	}()

	if err == nil {
		t.Fatal("expected an error for a step with no action set")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Fatalf("error %q should name the offending job", err.Error())
	}
}

// TestLowerMachineGroupDiskAndExt locks the MachineGroup mapping: ram_mb is
// bytes/2^20, disks[0].size_gb is bytes/2^30 with the already-lowered disk
// type, and ext_json is the group's Ext map marshaled as JSON.
func TestLowerMachineGroupDiskAndExt(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Provider: ast.ProviderUse{Use: "yandex"},
		Machines: map[string]ast.MachineGroup{
			"db": {
				Count: 3,
				Resources: ast.Resources{
					CPU:  4,
					RAM:  8 << 30,
					Disk: &ast.Disk{Size: 100 << 30, Type: "ssd"},
				},
				Ext: map[string]any{"platform_id": "standard-v3"},
			},
		},
		Services: map[string]ast.Service{},
	}
	provider := &ast.ProviderManifest{
		Name:     "yandex",
		Lowering: map[string]map[string]string{"disk.type": {"ssd": "network-ssd"}},
	}
	dom, diags := graph.Build(cluster, provider)
	if diags.HasErrors() {
		t.Fatalf("graph.Build: %+v", diags)
	}

	plan, err := lower.Lower(cluster, dom, map[string]ast.Job{})
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	if len(plan.GetMachineGroups()) != 1 {
		t.Fatalf("expected 1 machine group, got %d", len(plan.GetMachineGroups()))
	}
	mg := plan.GetMachineGroups()[0]
	if mg.GetName() != "db" || mg.GetCount() != 3 || mg.GetCpu() != 4 {
		t.Fatalf("unexpected group fields: %+v", mg)
	}
	if mg.GetRamMb() != 8<<10 { // 8 GiB in MiB
		t.Fatalf("RamMb = %d, want %d", mg.GetRamMb(), 8<<10)
	}
	if len(mg.GetDisks()) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(mg.GetDisks()))
	}
	if mg.GetDisks()[0].GetSizeGb() != 100 {
		t.Fatalf("SizeGb = %d, want 100", mg.GetDisks()[0].GetSizeGb())
	}
	if mg.GetDisks()[0].GetType() != "network-ssd" {
		t.Fatalf("disk type = %q, want the lowered %q", mg.GetDisks()[0].GetType(), "network-ssd")
	}
	if !strings.Contains(mg.GetExtJson(), "standard-v3") {
		t.Fatalf("ExtJson = %q, want it to contain %q", mg.GetExtJson(), "standard-v3")
	}
	if plan.GetProvider().GetName() != "yandex" {
		t.Fatalf("provider name = %q, want %q", plan.GetProvider().GetName(), "yandex")
	}
}

// TestLowerServiceHealthAndConfigs locks the ServiceSpec mapping, including
// Health -> HealthCheck (timeout rendered via time.Duration.String()) and
// Configs -> ConfigFile (template_path/dest).
func TestLowerServiceHealthAndConfigs(t *testing.T) {
	timeout, err := ast.ParseDuration("5s")
	if err != nil {
		t.Fatalf("ParseDuration: %v", err)
	}
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{},
		Services: map[string]ast.Service{
			"postgres": {
				On:      "db",
				Image:   "postgres:16",
				Network: "app",
				Volumes: []string{"/data"},
				Env:     map[string]string{"PGUSER": "stroppy"},
				Configs: []ast.ConfigFile{{Template: "pg.conf.tmpl", Dest: "/etc/postgresql/postgresql.conf"}},
				Health:  &ast.Health{HTTP: "http://db:5432/health", Timeout: timeout},
			},
		},
	}
	dom := plainDomain(cluster)

	plan, err := lower.Lower(cluster, dom, map[string]ast.Job{})
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	if len(plan.GetServices()) != 1 {
		t.Fatalf("expected 1 service, got %d", len(plan.GetServices()))
	}
	svc := plan.GetServices()[0]
	if svc.GetOnGroup() != "db" || svc.GetImage() != "postgres:16" || svc.GetNetwork() != "app" {
		t.Fatalf("unexpected service fields: %+v", svc)
	}
	if len(svc.GetConfigs()) != 1 || svc.GetConfigs()[0].GetTemplatePath() != "pg.conf.tmpl" || svc.GetConfigs()[0].GetDest() != "/etc/postgresql/postgresql.conf" {
		t.Fatalf("unexpected configs: %+v", svc.GetConfigs())
	}
	if svc.GetHealth().GetHttp() != "http://db:5432/health" || svc.GetHealth().GetTimeout() != "5s" {
		t.Fatalf("unexpected health: %+v", svc.GetHealth())
	}
}

// TestLowerSortsMachineGroupsServicesJobsByName locks the determinism
// contract: output order does not depend on Go's randomized map iteration.
func TestLowerSortsMachineGroupsServicesJobsByName(t *testing.T) {
	cluster := &ast.ClusterDoc{
		Machines: map[string]ast.MachineGroup{
			"zzz": {Count: 1},
			"aaa": {Count: 1},
			"mmm": {Count: 1},
		},
		Services: map[string]ast.Service{
			"zzz-svc": {},
			"aaa-svc": {},
		},
	}
	dom := plainDomain(cluster)
	jobs := map[string]ast.Job{
		"zzz-job": {Steps: []ast.Step{{Cmd: "true"}}},
		"aaa-job": {Steps: []ast.Step{{Cmd: "true"}}},
	}

	plan, err := lower.Lower(cluster, dom, jobs)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	gotGroups := make([]string, len(plan.GetMachineGroups()))
	for i, g := range plan.GetMachineGroups() {
		gotGroups[i] = g.GetName()
	}
	wantGroups := []string{"aaa", "mmm", "zzz"}
	if strings.Join(gotGroups, ",") != strings.Join(wantGroups, ",") {
		t.Fatalf("machine group order = %v, want %v", gotGroups, wantGroups)
	}

	gotServices := make([]string, len(plan.GetServices()))
	for i, s := range plan.GetServices() {
		gotServices[i] = s.GetName()
	}
	wantServices := []string{"aaa-svc", "zzz-svc"}
	if strings.Join(gotServices, ",") != strings.Join(wantServices, ",") {
		t.Fatalf("service order = %v, want %v", gotServices, wantServices)
	}

	gotJobs := make([]string, len(plan.GetJobs()))
	for i, j := range plan.GetJobs() {
		gotJobs[i] = j.GetId()
	}
	wantJobs := []string{"aaa-job", "zzz-job"}
	if strings.Join(gotJobs, ",") != strings.Join(wantJobs, ",") {
		t.Fatalf("job order = %v, want %v", gotJobs, wantJobs)
	}
}
