package recipe

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// pgListenPort is the port postgres listens on in the demo. The cluster runs on
// the default port; the workload connects through the placeholder port.
const pgListenPort = 5432

// Demo database/role created on the single node and used by the workload.
const (
	pgDemoDB   = "stroppy"
	pgDemoUser = "stroppy"
	pgDemoPass = "stroppy"
)

// pgRecipe is the single-node docker postgres deploy engine.
type pgRecipe struct{}

// BuildComponent dispatches per role.
func (pgRecipe) BuildComponent(role string, db map[string]any, wl map[string]any, refs Refs) *topology.Component_Strategy {
	switch expand.Role(role) {
	case expand.RoleDatabase:
		return pgDatabaseStrategy(db)
	case expand.RoleWorkload:
		return pgWorkloadStrategy(wl, refs)
	default:
		return nil
	}
}

// pgMajor reads pg_major from the baked config, defaulting to 16.
func pgMajor(db map[string]any) int {
	if v, ok := db[postgres.FieldPgMajor].(float64); ok {
		return int(v)
	}
	if v, ok := db[postgres.FieldPgMajor].(int); ok {
		return v
	}
	if v, ok := db[postgres.FieldPgMajor].(int64); ok {
		return int(v)
	}
	return int(postgres.PgMajor16)
}

// pgSyncReplicas returns the configured synchronous standby count (0 when
// synchronous mode is off or unset). Path: ha.patroni.synchronous_*.
func pgSyncReplicas(db map[string]any) int {
	ha, _ := db[postgres.FieldHA].(map[string]any)
	pat, _ := ha[postgres.FieldPatroni].(map[string]any)
	if pat == nil {
		return 0
	}
	mode, _ := pat[postgres.FieldSynchronousMode].(string)
	if mode == "" || mode == postgres.SyncModeOff {
		return 0
	}
	switch v := pat[postgres.FieldSynchronousNodeCount].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 1
}

// pgTuning reads a tuning knob from db.tuning, returning ok=false when absent.
func pgTuning(db map[string]any, field string) (string, bool) {
	tuning, ok := db[postgres.FieldTuning].(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := tuning[field].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// pgDatabaseStrategy emits the config files + install/config commands for the
// single postgres node.
func pgDatabaseStrategy(db map[string]any) *topology.Component_Strategy {
	major := pgMajor(db)

	return &topology.Component_Strategy{
		ConfigurationFiles: []*common.BakedFile{
			textFile("/etc/postgresql/postgresql.conf", 0o644, renderPostgresConf(db)),
			textFile("/etc/postgresql/pg_hba.conf", 0o640, renderPgHBA()),
		},
		DeploymentCommands: []*common.Cmd{
			pgInstallCmd(major),
			pgApplyConfCmd(major),
			pgStartAndSeedCmd(major),
		},
	}
}

// renderPostgresConf renders postgresql.conf: bind on all interfaces, default
// port, plus a couple of tuning values from the baked config when present.
func renderPostgresConf(db map[string]any) string {
	var b strings.Builder
	b.WriteString("# stroppy demo postgresql.conf (single-node docker)\n")
	b.WriteString("listen_addresses = '__LISTEN__'\n")
	fmt.Fprintf(&b, "port = %d\n", pgListenPort)
	b.WriteString("max_connections = 200\n")

	if v, ok := pgTuning(db, postgres.FieldSharedBuffers); ok {
		fmt.Fprintf(&b, "shared_buffers = '%s'\n", v)
	}
	if v, ok := pgTuning(db, postgres.FieldEffectiveCacheSize); ok {
		fmt.Fprintf(&b, "effective_cache_size = '%s'\n", v)
	}
	if v, ok := pgTuning(db, postgres.FieldWorkMem); ok {
		fmt.Fprintf(&b, "work_mem = '%s'\n", v)
	}

	return b.String()
}

// renderPgHBA renders a permissive demo pg_hba.conf (trust from anywhere). This
// is intentional for the single-node throwaway demo container; not for prod.
func renderPgHBA() string {
	return strings.Join([]string{
		"# stroppy demo pg_hba.conf (single-node docker, trust)",
		"local   all   all                  trust",
		"host    all   all   127.0.0.1/32   trust",
		"host    all   all   0.0.0.0/0      trust",
		"host    all   all   ::/0           trust",
		"",
	}, "\n")
}

// pgInstallCmd installs the postgres server + client for the requested major.
// Adds the PGDG apt repo first so any major (e.g. 16) is available on a base
// ubuntu image whose default repos only carry an older postgres.
func pgInstallCmd(major int) *common.Cmd {
	return scriptCmd(fmt.Sprintf(`set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl ca-certificates gnupg lsb-release
install -d /usr/share/postgresql-common/pgdg
curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list
apt-get update
apt-get install -y postgresql-%d postgresql-client-%d
`, major, major))
}

// pgApplyConfCmd copies the rendered confs into the cluster conf dir and
// substitutes the listen placeholder. Idempotent: re-running just re-copies.
func pgApplyConfCmd(major int) *common.Cmd {
	// Do NOT overwrite the package postgresql.conf (it carries data_directory,
	// include_dir, etc.); drop our overrides into the cluster's conf.d include
	// dir and replace only pg_hba.conf.
	return scriptCmd(fmt.Sprintf(`set -euo pipefail
CONFDIR="/etc/postgresql/%d/main"
mkdir -p "$CONFDIR/conf.d"
# bind on all interfaces for the demo
sed 's/__LISTEN__/*/g' /etc/postgresql/postgresql.conf > "$CONFDIR/conf.d/stroppy.conf"
cp /etc/postgresql/pg_hba.conf "$CONFDIR/pg_hba.conf"
chown -R postgres:postgres "$CONFDIR/conf.d" "$CONFDIR/pg_hba.conf"
`, major))
}

// pgStartAndSeedCmd starts the cluster, waits until ready, then creates the demo
// role + database. Idempotent: guards on existence before creating.
func pgStartAndSeedCmd(major int) *common.Cmd {
	return scriptCmd(fmt.Sprintf(`set -euo pipefail
pg_ctlcluster %d main start || pg_ctlcluster %d main restart
# wait until the server accepts connections
for i in $(seq 1 30); do
  if pg_isready -h 127.0.0.1 -p %d; then break; fi
  sleep 1
done
pg_isready -h 127.0.0.1 -p %d
# seed demo role + database (idempotent)
su - postgres -c "psql -tc \"SELECT 1 FROM pg_roles WHERE rolname='%s'\" | grep -q 1 || psql -c \"CREATE ROLE %s LOGIN SUPERUSER PASSWORD '%s'\""
su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='%s'\" | grep -q 1 || psql -c \"CREATE DATABASE %s OWNER %s\""
`, major, major, pgListenPort, pgListenPort,
		pgDemoUser, pgDemoUser, pgDemoPass,
		pgDemoDB, pgDemoDB, pgDemoUser))
}

// pgWorkloadStrategy emits the stroppy runner steps: download the stroppy binary
// (self-contained curl script for the demo) then run it against the DB host.
func pgWorkloadStrategy(wl map[string]any, refs Refs) *topology.Component_Strategy {
	_ = wl // workload config not consumed in the demo; reserved for tuning runs.
	return &topology.Component_Strategy{
		DeploymentCommands: []*common.Cmd{
			pgFetchStroppyCmd(refs),
			pgRunStroppyCmd(),
		},
	}
}

// pgFetchStroppyCmd downloads the stroppy binary by ref and installs it. Kept
// self-contained (curl) so the recipe needs no separate FetchFile activity; the
// agent could equally fetch the same ref via FetchFileActivity.
func pgFetchStroppyCmd(refs Refs) *common.Cmd {
	verify := ""
	if refs.StroppyChecksum != "" {
		verify = fmt.Sprintf(`echo "%s  /usr/local/bin/stroppy" | sha256sum -c -
`, refs.StroppyChecksum)
	}
	return scriptCmd(fmt.Sprintf(`set -euo pipefail
curl -fsSL "%s" -o /usr/local/bin/stroppy
chmod +x /usr/local/bin/stroppy
%s`, refs.StroppyBinaryURL, verify))
}

// pgRunStroppyCmd runs the stroppy workload against the deployed DB. The host and
// port are placeholders the workflow substitutes from the topology.
func pgRunStroppyCmd() *common.Cmd {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		pgDemoUser, pgDemoPass, PlaceholderDBHost, PlaceholderDBPort, pgDemoDB)
	return scriptCmd(fmt.Sprintf(`set -euo pipefail
/usr/local/bin/stroppy --url "%s" run
`, dsn))
}

// Wire produces the connections for the deployed topology. For the single-node
// demo: one workload→database FLOW connection over TCP on the postgres port.
func (pgRecipe) Wire(componentIDsByRole map[string][]string) []*topology.Connection {
	dbs := componentIDsByRole[string(expand.RoleDatabase)]
	workloads := componentIDsByRole[string(expand.RoleWorkload)]
	if len(dbs) == 0 || len(workloads) == 0 {
		return nil
	}

	port := uint32(pgListenPort)
	var conns []*topology.Connection
	for _, wl := range workloads {
		for _, db := range dbs {
			conns = append(conns, &topology.Connection{
				From:     wl,
				To:       db,
				Kind:     topology.Connection_KIND_FLOW,
				Protocol: topology.Connection_PROTOCOL_TCP,
				Mode:     topology.Connection_MODE_REQUEST,
				Port:     &port,
			})
		}
	}
	return conns
}
