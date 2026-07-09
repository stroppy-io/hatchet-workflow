package orioledb

import (
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
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

func TestMasterUnitHasStreamingFlags(t *testing.T) {
	opts := streamingMasterOptions(map[string]string{})
	for _, k := range []string{"wal_level", "max_wal_senders", "max_replication_slots", "hot_standby"} {
		if _, ok := opts[k]; !ok {
			t.Fatalf("master options missing %q: %v", k, opts)
		}
	}
	if opts["wal_level"] != "replica" {
		t.Fatalf("wal_level = %q, want replica", opts["wal_level"])
	}
}

func TestReplicaUnitBasebackupStandby(t *testing.T) {
	unit := orioledbReplicaUnit("orioledb-replica-1", "orioledb/orioledb:latest-pg17", "10.0.0.5", map[string]string{"hot_standby": "on"})
	for _, want := range []string{
		"docker run",
		"--network host",
		"pg_basebackup",
		"-h 10.0.0.5",
		"standby.signal",
		"docker-entrypoint.sh postgres",
		"orioledb/orioledb:latest-pg17",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("replica unit missing %q:\n%s", want, unit)
		}
	}
}

func TestHAProxyConfigSplitsWriteRead(t *testing.T) {
	cfg := orioledbHAProxyConfig([]string{"10.0.0.5", "10.0.0.6"}, map[string]string{})
	for _, want := range []string{
		"bind *:5432",
		"bind *:5433",
		"option httpchk GET /primary",
		"option httpchk GET /replica",
		"10.0.0.5:5432",
		"10.0.0.6:5432",
		"check port 8008",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("haproxy cfg missing %q:\n%s", want, cfg)
		}
	}
}

func TestHealthScriptUsesRecoveryCheck(t *testing.T) {
	s := orioledbHealthScript()
	for _, want := range []string{"pg_is_in_recovery", "/primary", "/replica", "200", "503"} {
		if !strings.Contains(s, want) {
			t.Fatalf("health script missing %q:\n%s", want, s)
		}
	}
}

func TestImageForVersion(t *testing.T) {
	cases := map[string]string{
		"pg16": "orioledb/orioledb:latest-pg16",
		"pg17": "orioledb/orioledb:latest-pg17",
		"pg18": "orioledb/orioledb:latest-pg18",
		"":     "orioledb/orioledb:latest-pg17", // empty -> defaultVersion
	}
	for in, want := range cases {
		if got := imageForVersion(in); got != want {
			t.Fatalf("imageForVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDBComponentImageFollowsVersion verifies the deployed image is derived
// solely from DatabaseParams.version, and that an empty version falls back to
// defaultVersion. There is no per-run image override.
func TestDBComponentImageFollowsVersion(t *testing.T) {
	unitFor := func(version string) string {
		db := &domain.Database{
			Source: &domain.Database_Params{Params: &domain.DatabaseParams{
				Version: version,
				Engine:  &domain.DatabaseParams_Orioledb{Orioledb: &domain.OrioledbParams{}},
			}},
		}
		comp := &topologypb.Component{Id: masterID, Engine: orioledbEngine, Role: orioledbRoleMaster}
		ec, err := orioledbDBComponent(comp, db, nil, nil, orioledbWiring{}, true)
		if err != nil {
			t.Fatalf("orioledbDBComponent(version=%q): %v", version, err)
		}
		return ec.ServiceFile.GetText()
	}

	if unit := unitFor("pg16"); !strings.Contains(unit, "orioledb/orioledb:latest-pg16") {
		t.Fatalf("pg16 version did not select latest-pg16 image:\n%s", unit)
	}
	if unit := unitFor("pg18"); !strings.Contains(unit, "orioledb/orioledb:latest-pg18") {
		t.Fatalf("pg18 version did not select latest-pg18 image:\n%s", unit)
	}
	if unit := unitFor(""); !strings.Contains(unit, "orioledb/orioledb:latest-pg17") {
		t.Fatalf("empty version did not fall back to latest-pg17:\n%s", unit)
	}
}

func TestServiceUnitRunsContainer(t *testing.T) {
	unit := orioledbServiceUnit("orioledb-master-1", "orioledb/orioledb:latest-pg17", "C", map[string]string{"shared_buffers": "512MB"}, "")
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
