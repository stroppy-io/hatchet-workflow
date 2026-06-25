package orioledb

import (
	"strings"
	"testing"

	topologypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestSupportsOnlyOrioledb(t *testing.T) {
	r := DeploymentRenderer{}
	if !r.Supports(&topologypb.Component{Engine: orioledbEngine}) {
		t.Fatal("must support orioledb engine")
	}
	if r.Supports(&topologypb.Component{Engine: "postgres"}) {
		t.Fatal("must not support postgres engine")
	}
}

func TestServiceUnitRunsContainer(t *testing.T) {
	unit := orioledbServiceUnit("orioledb-master-1", "orioledb/orioledb:latest-pg17", "C", map[string]string{"shared_buffers": "512MB"})
	for _, want := range []string{
		"docker run",
		"--network host",
		"orioledb/orioledb:latest-pg17",
		"POSTGRES_INITDB_ARGS=--locale=C",
		"ExecStartPre=", // docker pull
		"-c shared_buffers=512MB",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("service unit missing %q:\n%s", want, unit)
		}
	}
}
