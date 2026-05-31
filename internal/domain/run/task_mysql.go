package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// mysqlInstallTask installs the MySQL/MariaDB package on every DB node. The
// agent only runs the opaque apt script the server composes here.
type mysqlInstallTask struct {
	client   CommandSink
	state    *State
	version  string
	topology *types.MySQLTopology
	pkg      *types.Package
}

func (t *mysqlInstallTask) Execute(nc *NodeContext) error {
	targets := t.state.DBTargets()
	if t.pkg == nil {
		return fmt.Errorf("install mysql: no package provided")
	}
	nc.Log().Info("installing mysql on targets")

	// MySQL postinst tries to start/stop mysqld which fails in Docker. Install
	// with error tolerance, then verify the binary exists (the old agent ran
	// installPackage, and on error fell back to `which mysqld`). Both run inside
	// the same exclusive apt command so they serialize on the dpkg lock.
	installScript := installPackageScript(*t.pkg)
	tolerant := fmt.Sprintf("{\n%s\n} || which mysqld\n", installScript)
	return sendSeqAll(nc, t.client, targets,
		aptCmd("install_mysql", tolerant))
}

// mysqlConfigTask renders my.cnf per role, starts the cluster and wires up
// replication (async/semi-sync or Group Replication). All rendering and
// placeholder substitution happens here; the agent just writes files and runs
// the start/replication scripts.
type mysqlConfigTask struct {
	client    CommandSink
	state     *State
	topology  *types.MySQLTopology
	overrides map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "my.cnf:<role>"
}

func (t *mysqlConfigTask) Execute(nc *NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring mysql cluster")

	version := "8.0"

	// Build group seeds list (all nodes at port 33061) for Group Replication.
	var groupSeeds []string
	if t.topology.GroupRepl {
		for _, tgt := range targets {
			groupSeeds = append(groupSeeds, fmt.Sprintf("%s:33061", advertiseHost(tgt)))
		}
	}

	// Use a fixed UUID for group_replication_group_name.
	groupName := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	primaryHost := advertiseHost(targets[0])

	for i, target := range targets {
		role := "replica"
		if i == 0 {
			role = "primary"
		}
		// localHost is this node's own advertise address — used both inside the
		// rendered my.cnf body (when GR is on) and for placeholder substitution
		// on a user-supplied override.
		localHost := advertiseHost(target)

		spec := t.topology.Primary
		opts := t.topology.PrimaryOptions
		if role == "replica" {
			opts = t.topology.ReplicaOptions
			if len(t.topology.Replicas) > 0 {
				spec = t.topology.Replicas[0]
			}
		}

		// Config body: user override wins, else render server-side.
		body := t.overrides["my.cnf:"+role]
		if body == "" {
			body = dbconfig.RenderMySQLConf(dbconfig.RenderMySQLConfOpts{
				Version:       version,
				Role:          role,
				SemiSync:      t.topology.SemiSync,
				GroupRepl:     t.topology.GroupRepl,
				Options:       opts,
				TotalMemoryMB: spec.MemoryMB,
			})
		}
		// Per-node placeholder substitution (server-id, report_host) moves here
		// from the agent; node index is the loop index.
		body = dbconfig.SubstituteMySQLPlaceholders(body, i, localHost)

		cmds := []agent.Command{
			writeFile("config_mysql", "/etc/mysql/my.cnf", body),
			// Restart MySQL with our config. systemd handles startup correctly.
			runCmd("config_mysql", "systemctl restart mysql"),
			// Wait for MySQL to accept connections.
			runCmd("config_mysql", `for i in $(seq 1 30); do mysqladmin ping -h 127.0.0.1 -u root --silent 2>/dev/null && exit 0; sleep 1; done; echo "mysql not ready after 30s" >&2; journalctl -u mysql --no-pager -n 20 >&2; exit 1`),
			// Allow root login from any host. Use sql_log_bin=0 to avoid GTID conflicts when joining GR.
			runCmd("config_mysql", `mysql -u root -e "SET sql_log_bin=0; ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY ''; CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED WITH mysql_native_password BY ''; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION; FLUSH PRIVILEGES; SET sql_log_bin=1;"`),
		}

		if role == "primary" {
			// Create replication user on primary so replicas can connect.
			cmds = append(cmds, runCmd("config_mysql", `mysql -h 127.0.0.1 -u root -e "SET sql_log_bin=0; CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY 'repl_password'; GRANT REPLICATION SLAVE, BACKUP_ADMIN, GROUP_REPLICATION_STREAM ON *.* TO 'repl'@'%'; FLUSH PRIVILEGES; SET sql_log_bin=1;"`))
			// Create ProxySQL monitor user on primary.
			cmds = append(cmds, runCmd("config_mysql", `mysql -h 127.0.0.1 -u root -e "SET sql_log_bin=0; CREATE USER IF NOT EXISTS 'monitor'@'%' IDENTIFIED BY 'monitor'; GRANT USAGE ON *.* TO 'monitor'@'%'; FLUSH PRIVILEGES; SET sql_log_bin=1;"`))
		}

		// Group Replication setup via SQL (params cannot go in my.cnf — MySQL 8.0.45 rejects them before plugin load).
		if t.topology.GroupRepl {
			grSetup := fmt.Sprintf(`mysql -h 127.0.0.1 -u root -e "
			SET GLOBAL group_replication_group_name='%s';
			SET GLOBAL group_replication_local_address='%s:33061';
			SET GLOBAL group_replication_group_seeds='%s';
			SET GLOBAL group_replication_single_primary_mode=ON;
			SET GLOBAL group_replication_start_on_boot=OFF;
			CHANGE REPLICATION SOURCE TO SOURCE_USER='repl', SOURCE_PASSWORD='repl_password' FOR CHANNEL 'group_replication_recovery';
		"`, groupName, localHost, strings.Join(groupSeeds, ","))
			cmds = append(cmds, runCmd("config_mysql", grSetup))

			if role == "primary" {
				cmds = append(cmds, runCmd("config_mysql", `mysql -h 127.0.0.1 -u root -e "SET GLOBAL group_replication_bootstrap_group=ON; START GROUP_REPLICATION; SET GLOBAL group_replication_bootstrap_group=OFF;"`))
			} else {
				// Wait for primary GR bootstrap — check via root on primary (repl lacks performance_schema access).
				cmds = append(cmds, runCmd("config_mysql", fmt.Sprintf(`for i in $(seq 1 30); do
  mysql -h %s -u root -e "SELECT 1" 2>/dev/null && break
  sleep 1
done`, primaryHost)))
				cmds = append(cmds, runCmd("config_mysql", `mysql -h 127.0.0.1 -u root -e "START GROUP_REPLICATION;" 2>&1 || { echo "GR JOIN FAILED, MySQL error log:" >&2; tail -30 /var/log/mysql/error.log >&2 2>/dev/null; journalctl -u mysql --no-pager -n 15 >&2 2>/dev/null; exit 1; }`))
			}
		} else if role == "replica" && primaryHost != "" {
			// Standard async/semi-sync replication.
			// Wait for primary to be reachable before configuring replication.
			cmds = append(cmds, runCmd("config_mysql", fmt.Sprintf(`for i in $(seq 1 10); do mysql -h %s -u repl -prepl_password -e "SELECT 1" 2>/dev/null && break; sleep 1; done`, primaryHost)))
			cmds = append(cmds, runCmd("config_mysql", fmt.Sprintf(`mysql -h 127.0.0.1 -u root -e "CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_USER='repl', SOURCE_PASSWORD='repl_password', SOURCE_AUTO_POSITION=1; START REPLICA;"`, primaryHost)))
		}

		if err := sendSeq(nc, t.client, target, cmds...); err != nil {
			return err
		}
	}

	// Store effective config.
	p := t.topology.Primary
	ec := map[string]string{
		"kind":    "mysql",
		"primary": fmt.Sprintf("%d× %d vCPU / %d MB / %d GB", p.Count, p.CPUs, p.MemoryMB, p.DiskGB),
	}
	if len(t.topology.Replicas) > 0 {
		r := t.topology.Replicas[0]
		ec["replicas"] = fmt.Sprintf("%d× %d vCPU / %d MB", r.Count, r.CPUs, r.MemoryMB)
	}
	if t.topology.GroupRepl {
		ec["replication"] = "group"
	} else if t.topology.SemiSync {
		ec["replication"] = "semi-sync"
	}
	for k, v := range t.topology.PrimaryOptions {
		ec[k] = v
	}
	t.state.SetEffectiveConfig("database", ec)

	return nil
}
