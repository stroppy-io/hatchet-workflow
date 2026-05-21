package render

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
)

func comp(id string, k domain.Topology_Component_Kind) *domain.Topology_Component {
	return &domain.Topology_Component{Id: id, Kind: k}
}

func haTopo() *domain.Topology {
	return &domain.Topology{Machines: []*domain.Topology_Machine{
		{Id: "m1", Components: []*domain.Topology_Component{comp("etcd1", domain.Topology_Component_KIND_COORDINATOR)}},
		{Id: "m2", Components: []*domain.Topology_Component{comp("etcd2", domain.Topology_Component_KIND_COORDINATOR)}},
		{Id: "m3", Components: []*domain.Topology_Component{comp("db1", domain.Topology_Component_KIND_DATABASE)}},
		{Id: "m4", Components: []*domain.Topology_Component{comp("db2", domain.Topology_Component_KIND_DATABASE)}},
	}}
}

func patroniDB() *domain.Database {
	return &domain.Database{
		Kind: domain.Database_KIND_POSTGRES,
		Options: &domain.Database_Options{Options: &domain.Database_Options_Postgres_{Postgres: &domain.Database_Options_Postgres{
			Replication: &domain.Database_Options_Postgres_Replication{Mode: domain.Database_Options_Postgres_Replication_MODE_PATRONI},
		}}},
	}
}

func TestRenderPatroniBindsEtcdHosts(t *testing.T) {
	cfg, err := RenderComponent(comp("db1", domain.Topology_Component_KIND_DATABASE), patroniDB(), haTopo(), 4096)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	item := cfg.GetItems()[0]
	body := item.GetFile().GetContent().GetText()
	if !strings.Contains(body, "scope: stroppy-pg") || !strings.Contains(body, "__SELF_IP__:8008") {
		t.Errorf("patroni.yml unexpected:\n%s", body)
	}
	tokens := map[string]bool{}
	for _, b := range item.GetBindings() {
		tokens[b.GetToken()] = true
	}
	for _, want := range []string{"__SELF_IP__", "__ETCD_etcd1__", "__ETCD_etcd2__"} {
		if !tokens[want] {
			t.Errorf("missing binding %q", want)
		}
	}
}

func TestRenderHAProxyBindsDatabases(t *testing.T) {
	cfg := renderHAProxy(haTopo())
	item := cfg.GetItems()[0]
	if !strings.Contains(item.GetFile().GetContent().GetText(), "__DB_db1__:5432") {
		t.Errorf("haproxy backend missing db token:\n%s", item.GetFile().GetContent().GetText())
	}
	tokens := map[string]bool{}
	for _, b := range item.GetBindings() {
		tokens[b.GetToken()] = true
	}
	if !tokens["__DB_db1__"] || !tokens["__DB_db2__"] {
		t.Errorf("haproxy missing db bindings: %v", tokens)
	}
}

func mysqlReplTopo() *domain.Topology {
	return &domain.Topology{
		Machines: []*domain.Topology_Machine{
			{Id: "m1", Components: []*domain.Topology_Component{comp("db1", domain.Topology_Component_KIND_DATABASE)}},
			{Id: "m2", Components: []*domain.Topology_Component{comp("db2", domain.Topology_Component_KIND_DATABASE)}},
		},
		Connections: []*domain.Topology_Connection{
			{From: "db1", To: "db2", Kind: domain.Topology_Connection_KIND_REPLICATION},
		},
	}
}

func TestRenderMySQLReplicaGetsChangeMaster(t *testing.T) {
	db := &domain.Database{Kind: domain.Database_KIND_MYSQL}
	topo := mysqlReplTopo()

	replica, err := RenderComponent(comp("db2", domain.Topology_Component_KIND_DATABASE), db, topo, 4096)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var cmd *renderpb.Config_Item
	for _, it := range replica.GetItems() {
		if it.GetCommand() != nil {
			cmd = it
		}
	}
	if cmd == nil {
		t.Fatal("replica missing CHANGE REPLICATION SOURCE command")
	}
	if cmd.GetBindings()[0].GetComponentIds()[0] != "db1" {
		t.Errorf("replica command not bound to primary db1: %v", cmd.GetBindings())
	}

	primary, _ := RenderComponent(comp("db1", domain.Topology_Component_KIND_DATABASE), db, topo, 4096)
	var primaryCmd *renderpb.Config_Item
	for _, it := range primary.GetItems() {
		if it.GetCommand() != nil {
			primaryCmd = it
		}
	}
	// The primary provisions the replication user, not a CHANGE SOURCE.
	if primaryCmd == nil || primaryCmd.GetId() != "create_replica_user" {
		t.Errorf("primary should provision the replication user, got %v", primaryCmd.GetId())
	}
	if strings.Contains(primaryCmd.GetCommand().GetScript().GetText(), "CHANGE REPLICATION SOURCE") {
		t.Error("primary must not run CHANGE REPLICATION SOURCE")
	}
}

func TestRenderPicodataPeers(t *testing.T) {
	topo := &domain.Topology{Machines: []*domain.Topology_Machine{
		{Id: "m1", Components: []*domain.Topology_Component{comp("pd1", domain.Topology_Component_KIND_DATABASE)}},
		{Id: "m2", Components: []*domain.Topology_Component{comp("pd2", domain.Topology_Component_KIND_DATABASE)}},
	}}
	cfg, err := RenderComponent(comp("pd1", domain.Topology_Component_KIND_DATABASE),
		&domain.Database{Kind: domain.Database_KIND_PICODATA}, topo, 4096)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := cfg.GetItems()[0].GetFile().GetContent().GetText()
	if !strings.Contains(body, "__SELF_IP__:3301") || !strings.Contains(body, "__PD_pd2__:3301") {
		t.Errorf("picodata peers unexpected:\n%s", body)
	}
}

func TestRenderYDBClusterInitsOnFirstNode(t *testing.T) {
	topo := &domain.Topology{Machines: []*domain.Topology_Machine{
		{Id: "m1", Components: []*domain.Topology_Component{comp("ydb1", domain.Topology_Component_KIND_DATABASE)}},
		{Id: "m2", Components: []*domain.Topology_Component{comp("ydb2", domain.Topology_Component_KIND_DATABASE)}},
		{Id: "m3", Components: []*domain.Topology_Component{comp("ydb3", domain.Topology_Component_KIND_DATABASE)}},
	}}
	db := &domain.Database{Kind: domain.Database_KIND_YDB}

	cmdIDs := func(cfg *renderpb.Config) map[string]bool {
		out := map[string]bool{}
		for _, it := range cfg.GetItems() {
			if it.GetCommand() != nil {
				out[it.GetId()] = true
			}
		}
		return out
	}

	// Every node starts storage; only the first waits for the cluster to be ready
	// (the static config self-bootstraps — no separate blobstorage/database init).
	first, _ := RenderComponent(comp("ydb1", domain.Topology_Component_KIND_DATABASE), db, topo, 8192)
	fc := cmdIDs(first)
	if !fc["start_storage"] || !fc["wait_ready"] {
		t.Errorf("first node missing storage/wait commands: %v", fc)
	}
	second, _ := RenderComponent(comp("ydb2", domain.Topology_Component_KIND_DATABASE), db, topo, 8192)
	sc := cmdIDs(second)
	if !sc["start_storage"] {
		t.Error("second node missing start_storage")
	}
	if sc["wait_ready"] {
		t.Errorf("only the first node waits for cluster readiness: %v", sc)
	}
}

func TestRenderEtcdInitialCluster(t *testing.T) {
	cfg := renderEtcd(comp("etcd1", domain.Topology_Component_KIND_COORDINATOR), haTopo())
	body := cfg.GetItems()[0].GetFile().GetContent().GetText()
	// self peer uses the self token; the other peer uses its own token.
	if !strings.Contains(body, "etcd1=http://__SELF_IP__:2380") || !strings.Contains(body, "etcd2=http://__ETCD_etcd2__:2380") {
		t.Errorf("etcd initial-cluster unexpected:\n%s", body)
	}
}
