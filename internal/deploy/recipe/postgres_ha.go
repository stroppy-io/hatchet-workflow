package recipe

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// Ports for the postgres HA stack (mirrors main).
const (
	etcdClientPort   = 2379
	etcdPeerPort     = 2380
	patroniRESTPort  = 8008
	haproxyWritePort = 5000
	haproxyReadPort  = 5001
	pgBouncerPort    = 6432

	etcdVersion = "3.5.17"
)

// pgNodes returns all postgres data nodes (database + replicas) in order: the
// primary first, then replicas. Patroni elects the leader among them via etcd.
func pgNodes(cl Cluster) []Node {
	return append(append([]Node{}, cl.Role[expand.RoleDatabase]...), cl.Role[expand.RoleReplica]...)
}

// nodeIndex returns id's position in nodes (0 if absent).
func nodeIndex(nodes []Node, id string) int {
	for i, n := range nodes {
		if n.ID == id {
			return i
		}
	}
	return 0
}

// --- etcd (coordinator / DCS) ---

// etcdSteps installs and starts one etcd member. All members must start ~together
// (the workflow runs the coordinator tier first; each member's --initial-cluster
// lists every peer), then form the cluster.
func etcdSteps(self Node, cl Cluster) []Step {
	members := cl.Role[expand.RoleCoordinator]
	idx := nodeIndex(members, self.ID)
	name := fmt.Sprintf("etcd%d", idx)

	var peers []string
	for i, m := range members {
		peers = append(peers, fmt.Sprintf("etcd%d=http://%s:%d", i, m.IP, etcdPeerPort))
	}
	initialCluster := strings.Join(peers, ",")

	install := fmt.Sprintf(`set -e
if [ ! -x /usr/local/bin/etcd ]; then
  curl -fsSL --retry 5 --retry-delay 3 "https://github.com/etcd-io/etcd/releases/download/v%[1]s/etcd-v%[1]s-linux-amd64.tar.gz" -o /tmp/etcd.tar.gz
  tar xzf /tmp/etcd.tar.gz -C /tmp
  cp /tmp/etcd-v%[1]s-linux-amd64/etcd /usr/local/bin/etcd
  cp /tmp/etcd-v%[1]s-linux-amd64/etcdctl /usr/local/bin/etcdctl
  chmod +x /usr/local/bin/etcd /usr/local/bin/etcdctl
  rm -rf /tmp/etcd*
fi`, etcdVersion)

	start := fmt.Sprintf(`set -e
mkdir -p /var/lib/etcd
systemctl stop etcd 2>/dev/null || true; systemctl reset-failed etcd 2>/dev/null || true
systemd-run --unit=etcd --remain-after-exit -- /usr/local/bin/etcd \
  --name=%[1]s \
  --initial-cluster='%[2]s' \
  --initial-cluster-state=new \
  --initial-cluster-token=stroppy-etcd \
  --listen-client-urls=http://0.0.0.0:%[3]d \
  --listen-peer-urls=http://0.0.0.0:%[4]d \
  --advertise-client-urls=http://%[5]s:%[3]d \
  --initial-advertise-peer-urls=http://%[5]s:%[4]d \
  --data-dir=/var/lib/etcd`, name, initialCluster, etcdClientPort, etcdPeerPort, self.IP)

	wait := fmt.Sprintf(`set -e
for i in $(seq 1 30); do etcdctl endpoint health --endpoints=http://localhost:%d 2>/dev/null && exit 0; sleep 2; done
echo "etcd not healthy" >&2; exit 1`, etcdClientPort)

	return []Step{
		aptStep("apt deps", "curl ca-certificates tar"),
		cmdStep("install etcd", install),
		cmdStep("start etcd", start),
		cmdStep("wait etcd", wait),
	}
}

// --- patroni (database / replica) ---

// pgPatroniSteps installs postgres + Patroni and starts Patroni pointed at the
// etcd quorum. Primary and replica are identical modulo name/connect_address;
// Patroni elects the leader via etcd.
func pgPatroniSteps(self Node, cl Cluster, db map[string]any) []Step {
	major := pgMajor(db)
	idx := nodeIndex(pgNodes(cl), self.ID)
	name := fmt.Sprintf("pg%d", idx)

	etcdHosts := make([]string, 0)
	for _, ip := range cl.IPs(expand.RoleCoordinator) {
		etcdHosts = append(etcdHosts, fmt.Sprintf("%s:%d", ip, etcdClientPort))
	}

	sync := "false"
	if v := pgSyncReplicas(db); v > 0 {
		sync = "true"
	}

	patroniYML := fmt.Sprintf(`scope: stroppy-pg
name: %[1]s
restapi:
  listen: 0.0.0.0:%[2]d
  connect_address: %[3]s:%[2]d
etcd3:
  hosts: %[4]s
bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    synchronous_mode: %[5]s
    postgresql:
      use_pg_rewind: true
      parameters:
        wal_level: replica
        hot_standby: 'on'
        max_wal_senders: 10
        max_replication_slots: 10
        max_connections: 200
  initdb:
    - encoding: UTF8
    - data-checksums
  pg_hba:
    - local all all trust
    - host all all 0.0.0.0/0 trust
    - host replication all 0.0.0.0/0 trust
postgresql:
  listen: 0.0.0.0:%[6]d
  connect_address: %[3]s:%[6]d
  data_dir: /var/lib/postgresql/%[7]d/main
  bin_dir: /usr/lib/postgresql/%[7]d/bin
  pgpass: /tmp/pgpass
  authentication:
    replication:
      username: postgres
    superuser:
      username: postgres
`, name, patroniRESTPort, self.IP, strings.Join(etcdHosts, ","), sync, pgListenPort, major)

	start := fmt.Sprintf(`set -e
pg_ctlcluster %[1]d main stop 2>/dev/null || true
rm -rf /var/lib/postgresql/%[1]d/main/*
mkdir -p /var/lib/postgresql/%[1]d/main && chown -R postgres:postgres /var/lib/postgresql/%[1]d
systemctl stop patroni 2>/dev/null || true; systemctl reset-failed patroni 2>/dev/null || true
systemd-run --unit=patroni --uid=postgres --gid=postgres -- patroni /etc/patroni/patroni.yml`, major)

	wait := fmt.Sprintf(`set -e
for i in $(seq 1 60); do curl -sf http://localhost:%d/health 2>/dev/null && exit 0; sleep 2; done
echo "patroni not ready" >&2; journalctl -u patroni --no-pager -n 50 >&2 || true; exit 1`, patroniRESTPort)

	return append(pgInstallSteps(major), []Step{
		cmdStep("install patroni", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y python3-pip python3-dev libpq-dev\npip3 install --break-system-packages 'patroni[etcd3]' psycopg2-binary || pip3 install 'patroni[etcd3]' psycopg2-binary"),
		writeStep("write patroni.yml", "/etc/patroni/patroni.yml", 0o644, patroniYML),
		cmdStep("start patroni", start),
		cmdStep("wait patroni", wait),
	}...)
}

// --- haproxy (proxy) ---

// haproxyPostgresSteps installs haproxy fronting the Patroni cluster: a write
// frontend (:5000 → current leader, health-checked via Patroni /primary) and a
// read frontend (:5001 → replicas via /replica).
func haproxyPostgresSteps(cl Cluster) []Step {
	nodes := pgNodes(cl)
	var writeServers, readServers strings.Builder
	for i, n := range nodes {
		fmt.Fprintf(&writeServers, "    server pg%d %s:%d check port %d\n", i, n.IP, pgListenPort, patroniRESTPort)
		fmt.Fprintf(&readServers, "    server pg%d %s:%d check port %d\n", i, n.IP, pgListenPort, patroniRESTPort)
	}

	cfg := fmt.Sprintf(`global
    maxconn 4096
    log stdout format raw local0
defaults
    mode tcp
    timeout connect 5s
    timeout client 30s
    timeout server 30s
    retries 3
frontend ft_write
    bind *:%[1]d
    default_backend bk_write
frontend ft_read
    bind *:%[2]d
    default_backend bk_read
backend bk_write
    option httpchk GET /primary
    http-check expect status 200
    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions
%[3]sbackend bk_read
    option httpchk GET /replica
    http-check expect status 200
    balance roundrobin
    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions
%[4]s`, haproxyWritePort, haproxyReadPort, writeServers.String(), readServers.String())

	return []Step{
		cmdStep("install haproxy", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get update\napt-get install -y haproxy"),
		writeStep("write haproxy.cfg", "/etc/haproxy/haproxy.cfg", 0o644, cfg),
		cmdStep("start haproxy", "set -e\npkill haproxy 2>/dev/null || true\nhaproxy -f /etc/haproxy/haproxy.cfg -D"),
	}
}

// --- pgbouncer (pooler) ---

// pgBouncerSteps installs pgbouncer pointed at the local postgres (colocated).
// stroppy does not route through it (kept for parity with main).
func pgBouncerSteps() []Step {
	ini := fmt.Sprintf(`[databases]
* = host=127.0.0.1 port=%d
[pgbouncer]
listen_addr = 0.0.0.0
listen_port = %d
auth_type = trust
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25
admin_users = postgres
pidfile = /var/run/pgbouncer/pgbouncer.pid
logfile = /var/log/pgbouncer/pgbouncer.log
`, pgListenPort, pgBouncerPort)

	start := `set -e
id -u pgbouncer >/dev/null 2>&1 || useradd -r -m -s /bin/false pgbouncer
mkdir -p /var/run/pgbouncer /var/log/pgbouncer
chown -R pgbouncer:pgbouncer /etc/pgbouncer /var/run/pgbouncer /var/log/pgbouncer 2>/dev/null || true
su -s /bin/bash pgbouncer -c "pgbouncer -d /etc/pgbouncer/pgbouncer.ini"`

	return []Step{
		cmdStep("install pgbouncer", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y pgbouncer"),
		writeStep("write pgbouncer.ini", "/etc/pgbouncer/pgbouncer.ini", 0o644, ini),
		writeStep("write userlist", "/etc/pgbouncer/userlist.txt", 0o640, "\"postgres\" \"\"\n"),
		cmdStep("start pgbouncer", start),
	}
}

// pgInstallSteps installs postgres major from PGDG (shared by the single-node and
// Patroni paths).
func pgInstallSteps(major int) []Step {
	return []Step{
		cmdStep("apt update", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get update"),
		cmdStep("install apt deps", "set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y curl ca-certificates gnupg lsb-release"),
		cmdStep("add PGDG key", "set -e\ninstall -d /usr/share/postgresql-common/pgdg\ncurl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg"),
		cmdStep("add PGDG repo", `set -e
echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list
apt-get update`),
		cmdStep(fmt.Sprintf("install postgresql-%d", major),
			fmt.Sprintf("set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get install -y postgresql-%d postgresql-client-%d", major, major)),
	}
}

// aptStep is a convenience for a plain apt-get install of space-separated pkgs.
func aptStep(name, pkgs string) Step {
	return cmdStep(name, fmt.Sprintf("set -e\nexport DEBIAN_FRONTEND=noninteractive\napt-get update\napt-get install -y %s", pkgs))
}
