package recipe

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// crRecipe is the CockroachDB deploy engine. Flat homogeneous cluster: every DB
// node has role database; the first node runs `cockroach init`.
type crRecipe struct{}

const (
	cockroachVersion = "24.2.4"
	cockroachSQLPort = 26257
	cockroachDB      = "defaultdb"
)

// BuildComponent / Wire are unused by the granular (Steps) path.
func (crRecipe) BuildComponent(string, map[string]any, map[string]any, Refs) *topology.Component_Strategy {
	return nil
}
func (crRecipe) Wire(map[string][]string) []*topology.Connection { return nil }

// Steps routes a cockroach node by role.
func (crRecipe) Steps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	switch self.Role {
	case expand.RoleDatabase:
		return cockroachNodeSteps(self, cl)
	case expand.RoleWorkload:
		host := cockroachHost(cl)
		dsn := fmt.Sprintf("postgresql://root@%s:%d/%s?sslmode=disable", host, cockroachSQLPort, cockroachDB)
		return stroppyRunSteps(driverPostgres, "tpcc/tx", dsn, refs)
	default:
		return nil
	}
}

// cockroachHost is the first database node (stroppy connects pg-wire on 26257).
func cockroachHost(cl Cluster) string {
	if n, ok := cl.First(expand.RoleDatabase); ok {
		return n.IP
	}
	return "127.0.0.1"
}

// cockroachNodeSteps installs cockroach, starts the node (joined to all peers) and
// — on the first node only — initializes the cluster.
func cockroachNodeSteps(self Node, cl Cluster) []Step {
	nodes := cl.Role[expand.RoleDatabase]

	// --join lists every peer except self.
	var peers []string
	for _, n := range nodes {
		if n.ID != self.ID {
			peers = append(peers, fmt.Sprintf("%s:%d", n.IP, cockroachSQLPort))
		}
	}
	joinFlag := ""
	if len(peers) > 0 {
		joinFlag = "--join=" + strings.Join(peers, ",") + " "
	}

	install := fmt.Sprintf(`set -e
if [ ! -x /usr/local/bin/cockroach ]; then
  mkdir -p /opt/cockroach
  curl -fSL --retry 5 --retry-delay 3 "https://binaries.cockroachdb.com/cockroach-v%[1]s.linux-amd64.tgz" | tar -xz --strip-components=1 -C /opt/cockroach
  ln -sf /opt/cockroach/cockroach /usr/local/bin/cockroach
fi
groupadd -f cockroach && (id -u cockroach >/dev/null 2>&1 || useradd cockroach -g cockroach -d /var/lib/cockroach -m)
mkdir -p /var/lib/cockroach && chown -R cockroach:cockroach /var/lib/cockroach`, cockroachVersion)

	start := fmt.Sprintf(`set -e
systemctl stop cockroach 2>/dev/null || true; systemctl reset-failed cockroach 2>/dev/null || true
systemd-run --unit=cockroach --uid=cockroach --gid=cockroach -- /usr/local/bin/cockroach start --insecure \
  --advertise-addr=%[1]s:%[2]d --listen-addr=0.0.0.0:%[2]d --http-addr=0.0.0.0:8080 \
  --store=/var/lib/cockroach --cache=.25 --max-sql-memory=.25 %[3]s--background
for i in $(seq 1 30); do (echo > /dev/tcp/localhost/%[2]d) 2>/dev/null && exit 0; sleep 1; done
echo "cockroach not listening" >&2; journalctl -u cockroach --no-pager -n 50 >&2 || true; exit 1`,
		self.IP, cockroachSQLPort, joinFlag)

	steps := []Step{
		aptStep("apt deps", "curl ca-certificates tar"),
		cmdStep("install cockroach", install),
		cmdStep("start cockroach", start),
	}

	// First database node initializes the cluster.
	if len(nodes) > 0 && nodes[0].ID == self.ID {
		initCmd := fmt.Sprintf(`set -e
for i in $(seq 1 20); do
  /usr/local/bin/cockroach init --insecure --host=localhost:%[1]d 2>&1 | tee /tmp/crdb-init.log
  grep -q "already been initialized" /tmp/crdb-init.log && exit 0
  grep -q "Cluster successfully initialized" /tmp/crdb-init.log && exit 0
  sleep 2
done
exit 1`, cockroachSQLPort)
		steps = append(steps, cmdStep("init cluster", initCmd))
	}
	return steps
}
