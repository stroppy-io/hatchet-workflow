package compile

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/topology"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// MySQL / MariaDB: the official images, datadir on the data disk, the
// rendered my.cnf dropped into conf.d, root@% with the well-known password
// through the image env, replicas configured by an initdb.d script.
//
// doc: hub.docker.com/_/mysql, hub.docker.com/_/mariadb — env, conf.d,
// docker-entrypoint-initdb.d; dev.mysql.com/doc/refman/8.4/en/
// change-replication-source-to.html; mariadb.com/kb/en/change-master-to.

const (
	mysqlPort        = 3306
	mysqlDataInner   = "/var/lib/mysql"
	mysqlConfPath    = "/etc/mysql/conf.d/zz-stroppy.cnf"
	mysqlInitPath    = "/docker-entrypoint-initdb.d/10-stroppy.sql"
	proxysqlPort     = 6033
	proxysqlConfPath = "/etc/proxysql.cnf"
	exporterUser     = "exporter"
	exporterPassword = "stroppy_exporter"
)

func mysqlRecipe(c *compilation) error {
	p := c.params()
	maria := c.in.Database.Kind == catalog.MariaDB
	mode := strParam(p, "replication", "async")
	switch {
	case mode == "group":
		return errs.Newf(errs.CodeInvalid, "params.replication: group replication is not launchable yet")
	case mode == "galera":
		return errs.Newf(errs.CodeInvalid, "params.replication: galera is not launchable yet")
	case boolParam(p, "maxscale"):
		return errs.Newf(errs.CodeInvalid, "params.maxscale: maxscale is not launchable yet")
	}
	image, err := c.imageOf()
	if err != nil {
		return err
	}
	confPrefix := "cfg.my.cnf@"
	if maria {
		confPrefix = "cfg.mariadb.cnf@"
	}
	primary := c.first(topology.RoleDB)
	serverID := 0
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			serverID++
			overlay := map[string]any{"server_id": serverID, "report_host": ip(m), "bind_address": "0.0.0.0", "port": mysqlPort}
			if !maria {
				overlay["gtid_mode"] = "ON"
				overlay["enforce_gtid_consistency"] = "ON"
			}
			conf, err := c.render(role, c.schemaFor(role, confPrefix), "conf", overlay)
			if err != nil {
				return err
			}
			ct := spec.Container{
				Name: m + "-" + c.engineOf(role), Role: role, Machine: m, Image: image,
				Env:    map[string]string{"MYSQL_ROOT_PASSWORD": mysqlPassword, "MYSQL_ROOT_HOST": "%", "MARIADB_ROOT_PASSWORD": mysqlPassword, "MARIADB_ROOT_HOST": "%"},
				Mounts: []spec.Mount{{Source: dataMount + "/mysql", Target: mysqlDataInner}},
				Files:  []spec.File{{Path: mysqlConfPath, Content: conf}},
				Ports:  []spec.Port{{Container: mysqlPort, Host: mysqlPort}},
				Healthcheck: healthcheck("CMD-SHELL",
					fmt.Sprintf("%s -h127.0.0.1 -uroot -p%s -e 'SELECT 1'", clientOf(maria), mysqlPassword)),
				Restart: "always",
			}
			if role == topology.RoleDB {
				ct.Env["MYSQL_DATABASE"], ct.Env["MARIADB_DATABASE"] = mysqlDatabase, mysqlDatabase
				ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: mysqlInitSQL(strParam(p, "init_sql", ""))})
			} else {
				ct.Files = append(ct.Files, spec.File{Path: mysqlInitPath, Content: mysqlReplicaSQL(maria, ip(primary))})
				ct.DependsOn = []string{primary + "-" + c.engineOf(topology.RoleDB)}
			}
			c.add(ct)
		}
	}
	for _, role := range []string{topology.RoleDB, topology.RoleDBReplica} {
		for _, m := range c.machinesOf(role) {
			c.add(spec.Container{
				Name: m + "-mysqld-exporter", Role: role, Machine: m, Image: imageMySQLDExporter,
				Env: map[string]string{"MYSQLD_EXPORTER_PASSWORD": exporterPassword},
				Cmd: []string{
					fmt.Sprintf("--mysqld.address=127.0.0.1:%d", mysqlPort), "--mysqld.username=" + exporterUser, fmt.Sprintf("--web.listen-address=:%d", mysqldExporterPort),
				},
				Ports:     []spec.Port{{Container: mysqldExporterPort, Host: mysqldExporterPort}},
				Scrape:    "/metrics",
				Restart:   "always",
				DependsOn: []string{m + "-" + c.engineOf(role)},
			})
		}
	}
	if len(c.machinesOf(topology.RoleProxy)) > 0 {
		return c.proxysql()
	}
	return nil
}

func clientOf(maria bool) string {
	if maria {
		return "mariadb"
	}
	return "mysql"
}

// mysqlInitSQL runs once on the primary: the exporter user and init SQL.
func mysqlInitSQL(extra string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';\n", exporterUser, exporterPassword)
	fmt.Fprintf(&b, "GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO '%s'@'%%';\n", exporterUser)
	b.WriteString("FLUSH PRIVILEGES;\n")
	if extra != "" {
		b.WriteString(strings.TrimSpace(extra))
		b.WriteString("\n")
	}
	return b.String()
}

// mysqlReplicaSQL points a fresh replica at the primary; GTID auto
// positioning replays everything the primary did, the stroppy database
// included.
func mysqlReplicaSQL(maria bool, primaryHost string) string {
	if maria {
		return fmt.Sprintf("CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='root', MASTER_PASSWORD='%s', MASTER_USE_GTID=slave_pos;\nSTART SLAVE;\nSET GLOBAL read_only = ON;\n",
			primaryHost, mysqlPort, mysqlPassword)
	}
	return fmt.Sprintf("CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='root', SOURCE_PASSWORD='%s', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1;\nSTART REPLICA;\nSET GLOBAL read_only = ON;\n",
		primaryHost, mysqlPort, mysqlPassword)
}

// proxysql renders proxysql.cnf with the primary in the writer hostgroup
// and the replicas in the reader hostgroup.
func (c *compilation) proxysql() error {
	role := topology.RoleProxy
	schemaID := c.schemaFor(role, "cfg.proxysql.cnf@")
	if schemaID == "" {
		return errs.Newf(errs.CodeInvalid, "%s: role %s has no proxysql schema", c.in.Database.Kind, role)
	}
	eff := c.effective(role, schemaID)
	writer, reader := int(numberOf(eff["writer_hostgroup"])), int(numberOf(eff["reader_hostgroup"]))
	if writer == 0 && reader == 0 {
		writer, reader = 10, 20
	}
	servers := []any{}
	for _, m := range c.machinesOf(topology.RoleDB) {
		servers = append(servers, map[string]any{"address": ip(m), "port": mysqlPort, "hostgroup": writer})
	}
	for _, m := range c.machinesOf(topology.RoleDBReplica) {
		servers = append(servers, map[string]any{"address": ip(m), "port": mysqlPort, "hostgroup": reader})
	}
	cnf, err := c.render(role, schemaID, "conf", map[string]any{
		"mysql_servers": servers, "username": mysqlUser, "password": mysqlPassword,
		"interfaces": fmt.Sprintf("0.0.0.0:%d", proxysqlPort), "monitor_username": exporterUser, "monitor_password": exporterPassword,
	})
	if err != nil {
		return err
	}
	for _, m := range c.machinesOf(role) {
		c.add(spec.Container{
			Name: m + "-proxysql", Role: role, Machine: m, Image: imageProxySQL,
			Files:       []spec.File{{Path: proxysqlConfPath, Content: cnf}},
			Ports:       []spec.Port{{Container: proxysqlPort, Host: proxysqlPort}},
			Healthcheck: healthcheck("CMD-SHELL", fmt.Sprintf("nc -z 127.0.0.1 %d", proxysqlPort)),
			Restart:     "always",
			DependsOn:   c.dbContainers(),
		})
	}
	return nil
}
