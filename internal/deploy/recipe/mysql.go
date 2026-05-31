package recipe

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// myRecipe is the MySQL / MariaDB deploy engine. maria switches the package +
// repo (the rest of the single-node path is identical). Group replication is a
// TODO (cluster topologies); single-node is a standalone primary.
type myRecipe struct{ maria bool }

const mysqlPort = 3306

func (myRecipe) BuildComponent(string, map[string]any, map[string]any, Refs) *topology.Component_Strategy {
	return nil
}
func (myRecipe) Wire(map[string][]string) []*topology.Connection { return nil }

func (r myRecipe) Steps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	switch self.Role {
	case expand.RoleDatabase:
		return mysqlNodeSteps(r.maria)
	case expand.RoleWorkload:
		host := "127.0.0.1"
		if n, ok := cl.First(expand.RoleDatabase); ok {
			host = n.IP
		}
		dsn := fmt.Sprintf("root@tcp(%s:%d)/%s", host, mysqlPort, pgDemoDB)
		return stroppyRunSteps(driverMySQL, "tpcc/tx", dsn, refs)
	default:
		return nil
	}
}

// mysqlNodeSteps installs a standalone MySQL/MariaDB primary and seeds a root@%
// account + the stroppy database.
func mysqlNodeSteps(maria bool) []Step {
	var repo, pkgs, svc string
	if maria {
		repo = `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl ca-certificates
curl -fsSL https://r.mariadb.com/downloads/mariadb_repo_setup -o /tmp/mariadb_repo_setup
bash /tmp/mariadb_repo_setup --mariadb-server-version=10.11
apt-get update`
		pkgs = "mariadb-server mariadb-client"
		svc = "mariadb"
	} else {
		repo = `set -e
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y curl ca-certificates gnupg lsb-release
install -d /etc/apt/keyrings
curl -fsSL https://repo.mysql.com/RPM-GPG-KEY-mysql-2023 | gpg --dearmor -o /etc/apt/keyrings/mysql.gpg
echo "deb [signed-by=/etc/apt/keyrings/mysql.gpg] http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) mysql-8.0" > /etc/apt/sources.list.d/mysql.list
apt-get update`
		pkgs = "mysql-server mysql-client"
		svc = "mysql"
	}

	myCnf := `[mysqld]
bind-address = 0.0.0.0
server-id = 1
max_connections = 500
`

	seed := fmt.Sprintf(`set -e
for i in $(seq 1 60); do mysqladmin ping -h 127.0.0.1 --silent 2>/dev/null && break; sleep 1; done
mysql -h 127.0.0.1 -u root <<'SQL'
ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '';
CREATE USER IF NOT EXISTS 'root'@'%%' IDENTIFIED WITH mysql_native_password BY '';
GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%' WITH GRANT OPTION;
CREATE DATABASE IF NOT EXISTS %s;
FLUSH PRIVILEGES;
SQL`, pgDemoDB)

	return []Step{
		cmdStep("add repo", repo),
		cmdStep(fmt.Sprintf("install %s", svc), fmt.Sprintf("set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y %s || true\nwhich mysqld", pkgs)),
		writeStep("write my.cnf", "/etc/mysql/my.cnf", 0o644, myCnf),
		cmdStep("start "+svc, fmt.Sprintf("set -e\nsystemctl enable %s 2>/dev/null || true\nsystemctl restart %s", svc, svc)),
		cmdStep("seed root + db", seed),
	}
}
