package orioledb

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

var dummyParams = domain.OrioledbParams{}

func TestBuildTopologySpecScaled(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.OrioledbParams{Replicas: 2, Haproxy: 1})
	if err != nil {
		t.Fatalf("BuildTopologySpec: %v", err)
	}
	roles := map[string]int{}
	for _, c := range spec.GetComponents() {
		roles[c.GetRole()]++
	}
	if roles["master"] != 1 || roles["replica"] != 2 || roles["haproxy"] != 1 {
		t.Fatalf("want 1 master/2 replica/1 haproxy, got %v", roles)
	}
	if len(spec.GetNodes()) != 4 {
		t.Fatalf("want 4 nodes, got %d", len(spec.GetNodes()))
	}
	reps := 0
	for _, cn := range spec.GetConnections() {
		if cn.GetToComponentId() == "orioledb-master-1" && cn.GetEndpointName() == "rep" {
			reps++
		}
	}
	if reps != 2 {
		t.Fatalf("want 2 replica->master streaming connections, got %d", reps)
	}
}

func TestBuildTopologySpecSingleNode(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&dummyParams)
	if err != nil {
		t.Fatalf("BuildTopologySpec: %v", err)
	}
	if len(spec.GetNodes()) != 1 {
		t.Fatalf("want 1 node, got %d", len(spec.GetNodes()))
	}
	if len(spec.GetComponents()) != 1 {
		t.Fatalf("want 1 component, got %d", len(spec.GetComponents()))
	}
	c := spec.GetComponents()[0]
	if c.GetEngine() != orioledbEngine || c.GetRole() != orioledbRoleMaster {
		t.Fatalf("unexpected component engine=%q role=%q", c.GetEngine(), c.GetRole())
	}
	if len(spec.GetConnections()) != 0 {
		t.Fatalf("single node must have no connections, got %d", len(spec.GetConnections()))
	}
}
