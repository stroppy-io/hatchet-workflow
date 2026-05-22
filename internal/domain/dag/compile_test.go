package dag

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	rtagent "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

func singlePreset() *domain.TestPreset {
	return &domain.TestPreset{
		Database: &domain.Database{Kind: domain.Database_KIND_POSTGRES, Version: "16"},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{
					Id: "db-1", Cores: 4, MemoryGb: 16, DiskGb: 100, DataDisksGb: []uint64{200},
					Components: []*domain.Topology_Component{{Id: "pg", Kind: domain.Topology_Component_KIND_DATABASE}},
				},
			},
		},
	}
}

func hasEdge(d *primitive.Dag, from, to string) bool {
	for _, e := range d.GetEdges() {
		if e.GetSource() == from && e.GetTarget() == to {
			return true
		}
	}
	return false
}

func TestRecipeProducesRealCommands(t *testing.T) {
	sub, err := BuildInstallDag(singlePreset(), "test-dag")
	if err != nil {
		t.Fatalf("build install: %v", err)
	}
	var scripts []string
	for _, n := range sub.GetNodes() {
		var cmd rtagent.Command
		if err := n.GetTaskState().GetInput().UnmarshalTo(&cmd); err != nil {
			continue
		}
		if s := cmd.GetOperation().GetRunCmd().GetScript(); s != nil {
			scripts = append(scripts, s.GetText())
		}
	}
	all := strings.Join(scripts, "\n")
	for _, want := range []string{"postgresql-16", "apt-get install", "systemctl enable --now postgresql"} {
		if !strings.Contains(all, want) {
			t.Errorf("install recipe missing %q:\n%s", want, all)
		}
	}
}

func TestGraphCompilesHATopology(t *testing.T) {
	comp := func(id string, k domain.Topology_Component_Kind) *domain.Topology_Component {
		return &domain.Topology_Component{Id: id, Kind: k, Config: &renderpb.Config{}}
	}
	preset := &domain.TestPreset{
		Database: &domain.Database{
			Kind: domain.Database_KIND_POSTGRES, Version: "16",
			Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
				Replication: &domain.Database_Options_Postgres_Replication{Mode: domain.Database_Options_Postgres_Replication_MODE_PATRONI},
			}}},
		},
		Topology: &domain.Topology{
			Machines: []*domain.Topology_Machine{
				{Id: "m-etcd", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("etcd1", domain.Topology_Component_KIND_COORDINATOR)}},
				{Id: "m-db1", Cores: 4, MemoryGb: 16, Components: []*domain.Topology_Component{comp("db1", domain.Topology_Component_KIND_DATABASE)}},
				{Id: "m-db2", Cores: 4, MemoryGb: 16, Components: []*domain.Topology_Component{comp("db2", domain.Topology_Component_KIND_DATABASE)}},
				{Id: "m-px", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("px", domain.Topology_Component_KIND_PROXY)}},
				{Id: "m-load", Cores: 2, MemoryGb: 4, Components: []*domain.Topology_Component{comp("load", domain.Topology_Component_KIND_STROPPY)}},
			},
			Connections: []*domain.Topology_Connection{
				{From: "db1", To: "etcd1", Kind: domain.Topology_Connection_KIND_COORDINATION},
				{From: "db1", To: "db2", Kind: domain.Topology_Connection_KIND_REPLICATION},
				{From: "load", To: "px", Kind: domain.Topology_Connection_KIND_FLOW},
			},
		},
	}
	sub, err := BuildInstallDag(preset, "test-dag")
	if err != nil {
		t.Fatalf("build install: %v", err)
	}
	have := map[string]bool{}
	for _, n := range sub.GetNodes() {
		have[n.GetId()] = true
	}
	for _, want := range []string{"etcd1.apt_install", "db1.start_service", "px.apt_install", "load.run_stroppy"} {
		if !have[want] {
			t.Errorf("missing node %q", want)
		}
	}
	// etcd must finish before any database starts (cross-rank edge).
	if !hasEdge(sub, "etcd1.start_service", "db1.pre_install_0") && !hasEdge(sub, "etcd1.start_service", "db1.apt_install") {
		t.Error("expected coordinator->database ordering edge")
	}
	// Patroni (not postgresql) is the started service for HA postgres.
	var patroni bool
	for _, n := range sub.GetNodes() {
		if n.GetId() == "db1.start_service" {
			var cmd rtagent.Command
			_ = n.GetTaskState().GetInput().UnmarshalTo(&cmd)
			if strings.Contains(cmd.GetOperation().GetRunCmd().GetScript().GetText(), "patroni") {
				patroni = true
			}
		}
	}
	if !patroni {
		t.Error("expected patroni service start for HA postgres")
	}
}

// scriptTexts returns every RUN_CMD script body emitted by the install dag.
func scriptTexts(t *testing.T, sub *primitive.Dag) []string {
	t.Helper()
	var out []string
	for _, n := range sub.GetNodes() {
		var cmd rtagent.Command
		if err := n.GetTaskState().GetInput().UnmarshalTo(&cmd); err != nil {
			continue
		}
		if s := cmd.GetOperation().GetRunCmd().GetScript(); s != nil {
			out = append(out, s.GetText())
		}
	}
	return out
}

func TestMonitorChainPerMachine(t *testing.T) {
	sub, err := BuildInstallDag(singlePreset(), "test-dag")
	if err != nil {
		t.Fatalf("build install: %v", err)
	}
	have := map[string]bool{}
	for _, n := range sub.GetNodes() {
		have[n.GetId()] = true
	}
	// Every machine gets a node_exporter + vmagent monitor chain (machine id "db-1").
	for _, want := range []string{
		"db-1.monitor_node_exporter",
		"db-1.monitor_db_exporter", // pg machine → postgres_exporter
		"db-1.monitor_vmagent_install",
		"db-1.monitor_vmagent_config",
		"db-1.monitor_vmagent_start",
	} {
		if !have[want] {
			t.Errorf("missing monitor node %q", want)
		}
	}
	// Monitor chain is sequential within the machine.
	if !hasEdge(sub, "db-1.monitor_node_exporter", "db-1.monitor_db_exporter") {
		t.Error("monitor chain not ordered node_exporter -> db_exporter")
	}

	all := strings.Join(scriptTexts(t, sub), "\n")
	// node_exporter fetched from the SERVER cache (not apt), installed to /usr/local/bin.
	if !strings.Contains(all, "/binary/node_exporter/") || !strings.Contains(all, "/usr/local/bin/node_exporter") {
		t.Error("monitor chain missing node_exporter install from server cache")
	}
	// vmagent fetched from the server cache (vmutils tarball -> /usr/local/bin/vmagent).
	if !strings.Contains(all, "/binary/vmagent/") || !strings.Contains(all, "/usr/local/bin/vmagent") {
		t.Error("monitor chain missing vmagent install from server cache")
	}
	// remote_write points at the server's VM ingest via ${STROPPY_SERVER_ADDR}/vm.
	if !strings.Contains(all, "-remoteWrite.url=${STROPPY_SERVER_ADDR}/vm/insert/0/prometheus/api/v1/write") {
		t.Error("vmagent remote_write URL not wired to ${STROPPY_SERVER_ADDR}/vm")
	}
	// run_id external label is present and scrape config targets node_exporter + postgres_exporter.
	if !strings.Contains(all, "run_id:") {
		t.Error("vmagent scrape config missing run_id external label")
	}
	if !strings.Contains(all, "localhost:9100") || !strings.Contains(all, "localhost:9187") {
		t.Error("vmagent scrape config missing node_exporter / postgres_exporter targets")
	}
	// postgres_exporter fetched from the server cache, pointed at the LOCAL db.
	if !strings.Contains(all, "/binary/postgres_exporter/") || !strings.Contains(all, "DATA_SOURCE_NAME=postgresql://postgres@localhost:5432") {
		t.Error("postgres_exporter not installed from server cache / pointed at local db")
	}
}

func TestStroppyRunHasOTLPEndpoint(t *testing.T) {
	preset := singlePreset()
	preset.Topology.Machines = append(preset.Topology.Machines, &domain.Topology_Machine{
		Id: "load-1", Cores: 2, MemoryGb: 4,
		Components: []*domain.Topology_Component{{Id: "load", Kind: domain.Topology_Component_KIND_STROPPY}},
	})
	sub, err := BuildInstallDag(preset, "test-dag")
	if err != nil {
		t.Fatalf("build install: %v", err)
	}
	var found bool
	for _, n := range sub.GetNodes() {
		if n.GetId() != "load.run_stroppy" {
			continue
		}
		var cmd rtagent.Command
		if err := n.GetTaskState().GetInput().UnmarshalTo(&cmd); err != nil {
			t.Fatalf("unmarshal run_stroppy: %v", err)
		}
		script := cmd.GetOperation().GetRunCmd().GetScript().GetText()
		// stroppy v5 takes a run config file (not --url) with an OTLP exporter to the
		// server's VM ingest + a run_id resource attribute for metric correlation.
		if !strings.Contains(script, `"otlp_http_endpoint":"${STROPPY_AGENT_SERVER}"`) ||
			!strings.Contains(script, `/vm/insert/0/opentelemetry/api/v1/push`) {
			t.Errorf("run_stroppy missing OTLP exporter endpoint/path:\n%s", script)
		}
		if !strings.Contains(script, "OTEL_RESOURCE_ATTRIBUTES") || !strings.Contains(script, "run_id=") {
			t.Errorf("run_stroppy missing run_id resource attribute:\n%s", script)
		}
		if !strings.Contains(script, "stroppy run -f /etc/stroppy/run-config.json") {
			t.Errorf("run_stroppy not invoking stroppy with a config file:\n%s", script)
		}
		if !strings.Contains(script, `"driver_type":"postgres"`) {
			t.Errorf("run_stroppy config missing pg driver:\n%s", script)
		}
		found = true
	}
	if !found {
		t.Fatal("no load.run_stroppy node found")
	}
}

func TestInstallSubDagUsesAgentCommands(t *testing.T) {
	sub, err := BuildInstallDag(singlePreset(), "test-dag")
	if err != nil {
		t.Fatalf("build install: %v", err)
	}
	if sub == nil || len(sub.GetNodes()) == 0 {
		t.Fatal("install sub_dag has no nodes")
	}
	for _, n := range sub.GetNodes() {
		ts := n.GetTaskState()
		if ts.GetHandlerName() != HandlerAgentCommand {
			t.Errorf("node %q handler = %q, want %q", n.GetId(), ts.GetHandlerName(), HandlerAgentCommand)
		}
		if ts.GetLocus() != primitive.Dag_Node_TaskState_EXECUTION_LOCUS_AGENT {
			t.Errorf("node %q must be agent-locus", n.GetId())
		}
	}
}
