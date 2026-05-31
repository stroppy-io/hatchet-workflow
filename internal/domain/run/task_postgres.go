package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// pgInstallTask installs the postgres package on every DB node. The agent only
// runs the opaque apt script the server composes here.
type pgInstallTask struct {
	client   agent.Client
	state    *State
	version  string
	topology *types.PostgresTopology
	pkg      *types.Package
}

func (t *pgInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if t.pkg == nil {
		return fmt.Errorf("install postgres: no package provided")
	}
	nc.Log().Info("installing postgres on targets")
	return sendSeqAll(nc, t.client, targets,
		aptCmd("install_postgres", installPackageScript(*t.pkg)))
}

// pgConfigTask renders postgresql.conf / pg_hba.conf per role and starts the
// cluster. All rendering happens here; the agent just writes files and runs
// the start/replication scripts.
type pgConfigTask struct {
	client    agent.Client
	state     *State
	version   string
	topology  *types.PostgresTopology
	overrides map[string]string // keys: "postgresql.conf:<role>", "pg_hba.conf"
}

func (t *pgConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring postgres cluster")

	version := t.version
	if version == "" {
		version = "16"
	}

	masterHost := targets[0].InternalHost
	if masterHost == "" {
		masterHost = targets[0].Host
	}

	confDir := fmt.Sprintf("/etc/postgresql/%s/main", version)
	confPath := confDir + "/postgresql.conf"
	hbaPath := confDir + "/pg_hba.conf"
	dataDir := fmt.Sprintf("/var/lib/postgresql/%s/main", version)

	for i, target := range targets {
		var role string
		spec := t.topology.Master
		opts := t.topology.MasterOptions
		if i == 0 {
			role = "master"
		} else {
			role = "replica"
			if len(t.topology.Replicas) > 0 {
				spec = t.topology.Replicas[0]
			}
			opts = t.topology.ReplicaOptions
		}

		// Config body: user override wins, else render server-side.
		confBody := t.overrides["postgresql.conf:"+role]
		if confBody == "" {
			confBody = dbconfig.RenderPostgresConf(dbconfig.RenderPostgresConfOpts{
				Version:       version,
				Role:          role,
				Options:       opts,
				Patroni:       t.topology.Patroni,
				TotalMemoryMB: spec.MemoryMB,
			})
		}
		hbaBody := t.overrides["pg_hba.conf"]
		if hbaBody == "" {
			hbaBody = dbconfig.PostgresPgHbaConf()
		}

		cmds := []agent.Command{
			appendFile("config_postgres", confPath, "\n\n# stroppy-agent overrides\n"+confBody),
			writeFile("config_postgres", hbaPath, hbaBody),
		}

		if role == "master" {
			startScript := fmt.Sprintf(`systemctl restart postgresql 2>/dev/null || pg_ctlcluster %s main start || true
for i in $(seq 1 10); do
  pg_isready -U postgres && break
  sleep 1
done`, version)
			cmds = append(cmds, runCmd("config_postgres", startScript))
		} else {
			replicaScript := fmt.Sprintf(`pg_ctlcluster %s main stop || true
rm -rf %s/*
sudo -u postgres pg_basebackup -h %s -D %s -U postgres -Fp -Xs -P -R
chown -R postgres:postgres %s
pg_ctlcluster %s main start`, version, dataDir, masterHost, dataDir, dataDir, version)
			cmds = append(cmds, runCmd("config_postgres", replicaScript))
		}

		if err := sendSeq(nc, t.client, target, cmds...); err != nil {
			return err
		}
	}

	// Store effective config.
	m := t.topology.Master
	ec := map[string]string{
		"kind":    "postgres",
		"version": version,
		"master":  fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", m.Count, m.CPUs, m.MemoryMB, m.DiskGB),
	}
	if len(t.topology.Replicas) > 0 {
		r := t.topology.Replicas[0]
		ec["replicas"] = fmt.Sprintf("%d× %d vCPU / %d MB", r.Count, r.CPUs, r.MemoryMB)
	}
	if t.topology.Patroni {
		ec["ha"] = "patroni + etcd"
	}
	if t.topology.PgBouncer {
		ec["pooler"] = "pgbouncer"
	}
	for k, v := range t.topology.MasterOptions {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	return nil
}
