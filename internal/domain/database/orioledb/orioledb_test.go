package orioledb

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

var dummyParams = domain.OrioledbParams{}

func TestBuilderDispatchesOrioledb(t *testing.T) {
	// Guards that the builder switch routes KIND_ORIOLEDB here. Import the
	// builder package locally to avoid an import cycle at file top.
	t.Skip("covered by package-level builder_test in internal/domain/database")
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
