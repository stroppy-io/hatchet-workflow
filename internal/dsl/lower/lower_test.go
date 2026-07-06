package lower_test

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/ast"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/graph"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/include"
	"github.com/stroppy-io/stroppy-cloud/internal/dsl/lower"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
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
	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs, nil)
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
	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs, nil)
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
	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs, nil)
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
		_, err = lower.Lower(cluster, plainDomain(cluster), jobs, nil)
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

	plan, err := lower.Lower(cluster, dom, map[string]ast.Job{}, nil)
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

	plan, err := lower.Lower(cluster, dom, map[string]ast.Job{}, nil)
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

	plan, err := lower.Lower(cluster, dom, jobs, nil)
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

// findJob returns the CompiledJob with the given id from plan, failing the
// test if it is absent.
func findJob(t *testing.T, plan *dslpb.CompiledPlan, id string) *dslpb.CompiledJob {
	t.Helper()
	for _, j := range plan.GetJobs() {
		if j.GetId() == id {
			return j
		}
	}
	t.Fatalf("job %q not found in plan (have: %v)", id, plan.GetJobs())
	return nil
}

// TestLowerFillsResolvedInputsAndTargetGroupFromBoundComponent locks I1's
// groundwork (see internal/dsl/lower's package doc): a CompiledJob that
// originated from an include component (its id carries the component's
// job-name prefix, e.g. "ha/install" for a component instantiated as job
// "ha" — see include.BoundComponent's godoc) gets its resolved_inputs/
// input_groups/target_group filled from that BoundComponent — scalar inputs
// (Type != "machine_group") stringified into resolved_inputs, the sole
// machine_group input's bound group name into both input_groups and
// target_group. A job with no matching BoundComponent (never produced by an
// `include:` job) gets all three left zero.
func TestLowerFillsResolvedInputsAndTargetGroupFromBoundComponent(t *testing.T) {
	jobs := map[string]ast.Job{
		"ha/install": {On: "db", Steps: []ast.Step{{Cmd: "ha-install"}}},
		"top":        {Steps: []ast.Step{{Cmd: "true"}}},
	}
	components := []include.BoundComponent{
		{
			Name: "ha",
			Doc: &ast.ComponentDoc{
				Inputs: map[string]ast.InputSpec{
					"nodes": {Type: "machine_group"},
					"count": {Type: "int"},
				},
			},
			Inputs: map[string]any{"nodes": "db", "count": 3},
		},
	}
	cluster := plainCluster()

	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs, components)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	haJob := findJob(t, plan, "ha/install")
	if got := haJob.GetInputGroups()["nodes"]; got != "db" {
		t.Fatalf("input_groups[nodes] = %q, want %q", got, "db")
	}
	if got := haJob.GetTargetGroup(); got != "db" {
		t.Fatalf("target_group = %q, want %q", got, "db")
	}
	if got := haJob.GetResolvedInputs()["count"]; got != "3" {
		t.Fatalf("resolved_inputs[count] = %q, want %q", got, "3")
	}

	topJob := findJob(t, plan, "top")
	if len(topJob.GetResolvedInputs()) != 0 {
		t.Fatalf("top-level job resolved_inputs = %+v, want empty", topJob.GetResolvedInputs())
	}
	if len(topJob.GetInputGroups()) != 0 {
		t.Fatalf("top-level job input_groups = %+v, want empty", topJob.GetInputGroups())
	}
	if topJob.GetTargetGroup() != "" {
		t.Fatalf("top-level job target_group = %q, want empty", topJob.GetTargetGroup())
	}
}

// TestLowerTargetGroupEmptyWithZeroOrManyMachineGroupInputs locks the "0 or
// >1 machine_group inputs -> target_group empty" half of the brief: a
// component with two machine_group inputs bound gets both recorded in
// input_groups, but target_group stays "" since there is no single input to
// pick.
func TestLowerTargetGroupEmptyWithZeroOrManyMachineGroupInputs(t *testing.T) {
	jobs := map[string]ast.Job{
		"link/setup": {Steps: []ast.Step{{Cmd: "link"}}},
	}
	components := []include.BoundComponent{
		{
			Name: "link",
			Doc: &ast.ComponentDoc{
				Inputs: map[string]ast.InputSpec{
					"a": {Type: "machine_group"},
					"b": {Type: "machine_group"},
				},
			},
			Inputs: map[string]any{"a": "db", "b": "app"},
		},
	}
	cluster := plainCluster()

	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs, components)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	job := findJob(t, plan, "link/setup")
	if job.GetInputGroups()["a"] != "db" || job.GetInputGroups()["b"] != "app" {
		t.Fatalf("input_groups = %+v, want a=db b=app", job.GetInputGroups())
	}
	if job.GetTargetGroup() != "" {
		t.Fatalf("target_group = %q, want empty with two machine_group inputs", job.GetTargetGroup())
	}
}

// TestLowerNestedIncludeAttributesJobToInnermostComponent locks
// componentForJob's "most specific match wins" rule: when one component's
// own fragment includes another (jobs prefixed "outer/inner/..."), a job
// under the nested prefix is attributed to the inner BoundComponent (Name
// "outer/inner"), not the outer one, even though the outer's Name is also a
// (shorter) prefix match.
func TestLowerNestedIncludeAttributesJobToInnermostComponent(t *testing.T) {
	jobs := map[string]ast.Job{
		"outer/inner/setup": {Steps: []ast.Step{{Cmd: "inner"}}},
	}
	components := []include.BoundComponent{
		{
			Name: "outer",
			Doc: &ast.ComponentDoc{
				Inputs: map[string]ast.InputSpec{"nodes": {Type: "machine_group"}},
			},
			Inputs: map[string]any{"nodes": "outer-group"},
		},
		{
			Name: "outer/inner",
			Doc: &ast.ComponentDoc{
				Inputs: map[string]ast.InputSpec{"nodes": {Type: "machine_group"}},
			},
			Inputs: map[string]any{"nodes": "inner-group"},
		},
	}
	cluster := plainCluster()

	plan, err := lower.Lower(cluster, plainDomain(cluster), jobs, components)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	job := findJob(t, plan, "outer/inner/setup")
	if got := job.GetTargetGroup(); got != "inner-group" {
		t.Fatalf("target_group = %q, want %q (innermost component, not %q)", got, "inner-group", "outer-group")
	}
}
