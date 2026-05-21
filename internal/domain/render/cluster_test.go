package render

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
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

func TestRenderEtcdInitialCluster(t *testing.T) {
	cfg := renderEtcd(comp("etcd1", domain.Topology_Component_KIND_COORDINATOR), haTopo())
	body := cfg.GetItems()[0].GetFile().GetContent().GetText()
	// self peer uses the self token; the other peer uses its own token.
	if !strings.Contains(body, "etcd1=http://__SELF_IP__:2380") || !strings.Contains(body, "etcd2=http://__ETCD_etcd2__:2380") {
		t.Errorf("etcd initial-cluster unexpected:\n%s", body)
	}
}
