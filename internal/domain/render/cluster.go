package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	renderpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/render"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

// RenderComponent renders one topology component's on-host config files +
// late-binding holes (preview == execution). Configs that reference other
// components (patroni->etcd, haproxy->databases, etcd peers) emit one binding per
// referenced component so the resolver fills each host's real IP at execute time.
// totalMemoryMB sizes percent-based engine defaults.
func RenderComponent(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology, totalMemoryMB int) (*renderpb.Config, error) {
	switch c.GetKind() {
	case domain.Topology_Component_KIND_DATABASE:
		switch db.GetKind() {
		case domain.Database_KIND_POSTGRES:
			if IsPatroniManaged(c) {
				return renderPatroni(c, topo, totalMemoryMB, pgVersion(db.GetVersion())), nil
			}
		case domain.Database_KIND_MYSQL, domain.Database_KIND_MARIADB:
			if isMultiDB(topo) {
				return renderMySQLComponent(c, db, topo, totalMemoryMB), nil
			}
		case domain.Database_KIND_COCKROACH:
			return renderCockroachComponent(c, topo, totalMemoryMB), nil
		case domain.Database_KIND_PICODATA:
			if isMultiDB(topo) {
				return renderPicodataComponent(c, topo, totalMemoryMB), nil
			}
		case domain.Database_KIND_YDB:
			return renderYDBComponent(c, topo), nil
		}
		return RenderDatabase(db, totalMemoryMB)
	case domain.Topology_Component_KIND_COORDINATOR:
		return renderEtcd(c, topo), nil
	case domain.Topology_Component_KIND_PROXY:
		return renderProxy(c, db, topo), nil
	case domain.Topology_Component_KIND_MONITOR:
		// node_exporter needs no config file.
		return &renderpb.Config{Id: "monitor"}, nil
	default:
		return &renderpb.Config{Id: "no-config"}, nil
	}
}

// Component role labels. CompileTopology stamps these onto Topology_Component.Tags so
// the cluster role is an EXPLICIT, user-visible property of the component — never
// re-derived from Database.Options (would ignore topology edits) nor from connections
// (a network/graph concern). Render and recipe read only the component's own role.
const (
	RoleLabelKey = "role"
	RolePrimary  = "primary" // sole writable node (mysql primary, cockroach seed)
	RoleReplica  = "replica" // follows a primary (mysql replica, cockroach joiner)
	RolePatroni  = "patroni" // postgres node supervised by Patroni + etcd
)

// ComponentRole returns the component's cluster role label (empty when unset).
func ComponentRole(c *domain.Topology_Component) string {
	return c.GetTags().GetLabels()[RoleLabelKey]
}

// IsPatroniManaged reports whether a DATABASE component is Patroni-managed, read from
// its explicit role label (set by CompileTopology), not from options or edges.
func IsPatroniManaged(c *domain.Topology_Component) bool {
	return ComponentRole(c) == RolePatroni
}

// primaryDatabaseID returns the id of the DATABASE component marked RolePrimary (the
// replication source / cluster seed), or "" if none — found by role, not by edges.
func primaryDatabaseID(topo *domain.Topology) string {
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			if c.GetKind() == domain.Topology_Component_KIND_DATABASE && ComponentRole(c) == RolePrimary {
				return c.GetId()
			}
		}
	}
	return ""
}

const selfIPToken = "__SELF_IP__"

// peerToken is a per-component late-binding hole, e.g. __ETCD_etcd1__.
func peerToken(prefix, id string) string { return fmt.Sprintf("__%s_%s__", prefix, id) }

func componentIDsByKind(topo *domain.Topology, kind domain.Topology_Component_Kind) []string {
	var ids []string
	for _, m := range topo.GetMachines() {
		for _, c := range m.GetComponents() {
			if c.GetKind() == kind {
				ids = append(ids, c.GetId())
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func fileItemBound(id, path, content string, bindings []*renderpb.Config_Binding) *renderpb.Config_Item {
	it := fileItem(id, path, content)
	it.Bindings = bindings
	return it
}

// ─── patroni (postgres HA) ────────────────────────────────────────────────────

// pgVersion defaults the postgres major version for patroni paths.
func pgVersion(v string) string {
	if v == "" {
		return "16"
	}
	return v
}

func renderPatroni(c *domain.Topology_Component, topo *domain.Topology, totalMemoryMB int, version string) *renderpb.Config {
	bindings := []*renderpb.Config_Binding{{Token: selfIPToken, ComponentIds: []string{c.GetId()}, Attr: AttrPrivateIP}}

	var etcdHosts []string
	for _, eid := range componentIDsByKind(topo, domain.Topology_Component_KIND_COORDINATOR) {
		tok := peerToken("ETCD", eid)
		etcdHosts = append(etcdHosts, tok+":2379")
		bindings = append(bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{eid}, Attr: AttrPrivateIP})
	}

	pg := map[string]string{"shared_buffers": "25%", "max_connections": "200"}
	resolveMemoryPercents(pg, totalMemoryMB)

	var b strings.Builder
	b.WriteString("# Generated by stroppy\n")
	b.WriteString("scope: stroppy-pg\n")
	fmt.Fprintf(&b, "name: %s\n\n", c.GetId())
	b.WriteString("restapi:\n  listen: 0.0.0.0:8008\n")
	fmt.Fprintf(&b, "  connect_address: %s:8008\n\n", selfIPToken)
	fmt.Fprintf(&b, "etcd3:\n  hosts: %s\n\n", strings.Join(etcdHosts, ","))
	// Generous DCS timeouts: a single co-located etcd has fsync latency spikes
	// under benchmark load, briefly making it unreachable to every patroni. With
	// the default ttl:30/retry:10 the leader demotes itself (shuts down postgres,
	// anti-split-brain) on a transient hiccup and kills the in-flight workload.
	// ttl:90/retry:30 rides out ~50s of etcd unavailability (constraint:
	// loop_wait + 2*retry_timeout <= ttl → 10 + 60 = 70 <= 90).
	b.WriteString("bootstrap:\n  dcs:\n    ttl: 90\n    loop_wait: 10\n    retry_timeout: 30\n")
	b.WriteString("    postgresql:\n      use_pg_rewind: true\n      parameters:\n")
	b.WriteString("        wal_level: replica\n        hot_standby: 'on'\n        max_wal_senders: 10\n        max_replication_slots: 10\n")
	for _, k := range sortedKeys(pg) {
		fmt.Fprintf(&b, "        %s: '%s'\n", k, pg[k])
	}
	// initdb + bootstrap users so the leader can be created from scratch.
	b.WriteString("  initdb:\n    - encoding: UTF8\n    - data-checksums\n")
	// trust auth (no password) to match the single-node pg_hba and stroppy's
	// password-less connection URL (postgresql://postgres@host/postgres). md5 here
	// made the workload fail SASL auth (28P01) since stroppy sends no password.
	// patroni's superuser/replication passwords below are still used by patroni's
	// own client connections; trust just lets postgres accept them regardless.
	b.WriteString("  pg_hba:\n    - local all all trust\n    - host all all 0.0.0.0/0 trust\n    - host replication all 0.0.0.0/0 trust\n")
	b.WriteString("  users:\n    admin:\n      password: stroppy-admin\n      options:\n        - createrole\n        - createdb\n\n")

	// postgresql runtime: paths for the debian package + auth so patroni manages it.
	bin := "/usr/lib/postgresql/" + version + "/bin"
	dataDir := "/var/lib/postgresql/" + version + "/main"
	fmt.Fprintf(&b, "postgresql:\n  listen: 0.0.0.0:5432\n  connect_address: %s:5432\n", selfIPToken)
	fmt.Fprintf(&b, "  data_dir: %s\n  bin_dir: %s\n  pgpass: /tmp/pgpass0\n", dataDir, bin)
	b.WriteString("  authentication:\n")
	b.WriteString("    replication:\n      username: replicator\n      password: stroppy-repl\n")
	b.WriteString("    superuser:\n      username: postgres\n      password: stroppy-super\n")

	return &renderpb.Config{Id: "patroni", Items: []*renderpb.Config_Item{
		fileItemBound("patroni.yml", "/etc/patroni/patroni.yml", b.String(), bindings),
	}}
}

// ─── etcd (coordinator) ───────────────────────────────────────────────────────

func renderEtcd(c *domain.Topology_Component, topo *domain.Topology) *renderpb.Config {
	bindings := []*renderpb.Config_Binding{{Token: selfIPToken, ComponentIds: []string{c.GetId()}, Attr: AttrPrivateIP}}

	var cluster []string
	for _, eid := range componentIDsByKind(topo, domain.Topology_Component_KIND_COORDINATOR) {
		tok := peerToken("ETCD", eid)
		cluster = append(cluster, fmt.Sprintf("%s=http://%s:2380", eid, tok))
		if eid != c.GetId() {
			bindings = append(bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{eid}, Attr: AttrPrivateIP})
		}
	}
	// self peer uses the self token (already bound).
	clusterStr := strings.ReplaceAll(strings.Join(cluster, ","), peerToken("ETCD", c.GetId()), selfIPToken)

	// The Debian/Ubuntu etcd-server service reads /etc/default/etcd env vars (NOT a
	// yaml config); listen on 0.0.0.0 so patroni in other VMs/containers can reach it.
	var b strings.Builder
	b.WriteString("# Generated by stroppy\n")
	fmt.Fprintf(&b, "ETCD_NAME=%s\n", c.GetId())
	b.WriteString("ETCD_DATA_DIR=/var/lib/etcd/default\n")
	b.WriteString("ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380\n")
	b.WriteString("ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379\n")
	fmt.Fprintf(&b, "ETCD_INITIAL_ADVERTISE_PEER_URLS=http://%s:2380\n", selfIPToken)
	fmt.Fprintf(&b, "ETCD_ADVERTISE_CLIENT_URLS=http://%s:2379\n", selfIPToken)
	fmt.Fprintf(&b, "ETCD_INITIAL_CLUSTER=%s\n", clusterStr)
	b.WriteString("ETCD_INITIAL_CLUSTER_STATE=new\n")
	b.WriteString("ETCD_INITIAL_CLUSTER_TOKEN=stroppy-etcd\n")
	b.WriteString("ETCD_ENABLE_V2=true\n")

	return &renderpb.Config{Id: "etcd", Items: []*renderpb.Config_Item{
		fileItemBound("etcd-default", "/etc/default/etcd", b.String(), bindings),
	}}
}

// ─── proxy (haproxy / proxysql) ───────────────────────────────────────────────

func renderProxy(_ *domain.Topology_Component, db *domain.Database, topo *domain.Topology) *renderpb.Config {
	if db.GetKind() == domain.Database_KIND_MYSQL || db.GetKind() == domain.Database_KIND_MARIADB {
		return renderProxySQL(topo)
	}
	return renderHAProxy(topo)
}

func renderHAProxy(topo *domain.Topology) *renderpb.Config {
	var bindings []*renderpb.Config_Binding
	var servers strings.Builder
	for _, dbid := range componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE) {
		tok := peerToken("DB", dbid)
		// httpchk against the patroni REST API (8008) routes writes to the leader.
		fmt.Fprintf(&servers, "    server %s %s:5432 check port 8008\n", dbid, tok)
		bindings = append(bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{dbid}, Attr: AttrPrivateIP})
	}
	var b strings.Builder
	b.WriteString("# Generated by stroppy\nglobal\n  maxconn 4096\ndefaults\n  mode tcp\n  timeout connect 5s\n  timeout client 30m\n  timeout server 30m\n\n")
	b.WriteString("frontend pg\n  bind 0.0.0.0:5432\n  default_backend pg_primary\n\n")
	b.WriteString("backend pg_primary\n  option httpchk GET /primary\n  http-check expect status 200\n")
	b.WriteString(servers.String())
	return &renderpb.Config{Id: "haproxy", Items: []*renderpb.Config_Item{
		fileItemBound("haproxy.cfg", "/etc/haproxy/haproxy.cfg", b.String(), bindings),
	}}
}

func renderProxySQL(topo *domain.Topology) *renderpb.Config {
	var bindings []*renderpb.Config_Binding
	var hosts strings.Builder
	for i, dbid := range componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE) {
		tok := peerToken("DB", dbid)
		// hostgroup 0 = writer (first/primary), 1 = readers.
		hg := 1
		if i == 0 {
			hg = 0
		}
		fmt.Fprintf(&hosts, "  { address=\"%s\", port=3306, hostgroup=%d, max_connections=1000 },\n", tok, hg)
		bindings = append(bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{dbid}, Attr: AttrPrivateIP})
	}
	var b strings.Builder
	b.WriteString("# Generated by stroppy\ndatadir=\"/var/lib/proxysql\"\n\n")
	b.WriteString("admin_variables=\n{\n  admin_credentials=\"admin:admin\"\n  mysql_ifaces=\"0.0.0.0:6032\"\n}\n\n")
	// Listen on 3306 so the workload's standard mysql URL (host:3306) reaches the
	// proxy transparently (the proxy node runs no mysqld, so 3306 is free). monitor
	// uses the workload `stroppy` user (the backend has no separate monitor user).
	b.WriteString("mysql_variables=\n{\n  threads=4\n  interfaces=\"0.0.0.0:3306\"\n  monitor_username=\"stroppy\"\n  monitor_password=\"\"\n}\n\n")
	b.WriteString("mysql_servers=\n(\n")
	b.WriteString(hosts.String())
	b.WriteString(")\n\n")
	// proxysql must know the client user to authenticate + route it. Route everything
	// to hostgroup 0 (the primary/writer) — no read-split, so no replica-lag surprises
	// for the tpcc consistency checks.
	b.WriteString("mysql_users=\n(\n  { username=\"stroppy\", password=\"\", default_hostgroup=0, default_schema=\"stroppy\", max_connections=1000, active=1 }\n)\n")
	return &renderpb.Config{Id: "proxysql", Items: []*renderpb.Config_Item{
		fileItemBound("proxysql.cnf", "/etc/proxysql.cnf", b.String(), bindings),
	}}
}

// ─── ydb cluster (static storage + blobstorage init + database) ───────────────
//
// TODO(render): the ydb config.yaml skeleton + ydbd invocation are structurally
// modeled (hosts/erasure/init), but the exact YDB config schema + flags must be
// verified against a real ydb cluster (no ydb available to test here).
func renderYDBComponent(c *domain.Topology_Component, topo *domain.Topology) *renderpb.Config {
	dbs := componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE)
	const pdisk = "/opt/ydb/data/pdisk1.data"
	isFirst := len(dbs) > 0 && dbs[0] == c.GetId()

	// YDB topology here is compute/storage-separated: the FIRST node is the single static
	// STORAGE node (erasure "none", one file pdisk) that bootstraps the cluster + the tenant
	// /Root/db1; EVERY node runs a dynamic COMPUTE node serving that tenant. A real
	// fault-tolerant multi-node STORAGE group (mirror-3 / mirror-3-dc) can't form on Docker
	// file pdisks — mirror-3-dc panics in CreateMirror3dcMapper (wants a ~9-disk realm/domain
	// layout) and mirror-3 panics on the first VPut ("ingress mismatch") when a node writes
	// before its peers join; and a multi-node tenant schemeshard never gets placed ("could
	// not resolve redirected path"). Storage redundancy needs the cloud path (raw block
	// devices). So storage is single-node (the proven single config) and the cluster's value
	// is the extra COMPUTE nodes. The config below describes that one storage node; nodes
	// reach it for node-broker via the __YDB_<first>__ token (its private IP).
	node1Tok := selfIPToken
	if !isFirst {
		node1Tok = peerToken("YDB", dbs[0])
	}
	bindings := []*renderpb.Config_Binding{{Token: selfIPToken, ComponentIds: []string{c.GetId()}, Attr: AttrPrivateIP}}
	if !isFirst {
		bindings = append(bindings, &renderpb.Config_Binding{Token: node1Tok, ComponentIds: []string{dbs[0]}, Attr: AttrPrivateIP})
	}

	var b strings.Builder
	b.WriteString("# Generated by stroppy\n")
	b.WriteString("static_erasure: none\n")
	b.WriteString("host_configs:\n  - host_config_id: 1\n    drive:\n      - path: " + pdisk + "\n        type: SSD\n")
	fmt.Fprintf(&b, "hosts:\n  - host: %s\n    node_id: 1\n    host_config_id: 1\n    walle_location:\n      body: 1\n      data_center: 'zone-a'\n      rack: '1'\n", node1Tok)
	b.WriteString("domains_config:\n  domain:\n    - name: Root\n      storage_pool_types:\n")
	b.WriteString("        - kind: ssd\n          pool_config:\n            box_id: 1\n            erasure_species: none\n")
	b.WriteString("            kind: ssd\n            pdisk_filter:\n              - property:\n                  - type: SSD\n            vdisk_kind: Default\n")
	b.WriteString("  state_storage:\n    - ring:\n        node: [1]\n        nto_select: 1\n      ssid: 1\n")
	b.WriteString("table_service_config:\n  sql_version: 1\n")
	b.WriteString("actor_system_config:\n  use_auto_config: true\n  node_type: STORAGE\n")
	b.WriteString("blob_storage_config:\n  service_set:\n    groups:\n      - erasure_species: none\n        rings:\n          - fail_domains:\n")
	fmt.Fprintf(&b, "              - vdisk_locations:\n                  - node_id: 1\n                    pdisk_category: SSD\n                    path: %s\n", pdisk)
	b.WriteString("channel_profile_config:\n  profile:\n    - channel:\n")
	for range 3 {
		b.WriteString("        - erasure_species: none\n          pdisk_category: 0\n          storage_pool_kind: ssd\n")
	}

	items := []*renderpb.Config_Item{fileItemBound("ydb-config.yaml", "/etc/ydb/config.yaml", b.String(), bindings)}

	if isFirst {
		// start_storage: format the file pdisk + run the single STATIC storage node (grpc
		// 2135, --node 1). Idempotent across the 60s lease re-delivery: only format + start
		// when 2135 is not already serving (never re-format a live pdisk / collide on the unit).
		items = append(items, commandItem("start_storage", strings.Join([]string{
			"install -d /opt/ydb/data",
			"if ! (echo > /dev/tcp/127.0.0.1/2135) 2>/dev/null; then",
			"  truncate -s 8G " + pdisk,
			"  LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd admin bs disk obliterate " + pdisk + " 2>/dev/null || true",
			"  systemctl reset-failed stroppy-ydb-storage >/dev/null 2>&1 || true",
			"  systemctl is-active --quiet stroppy-ydb-storage || systemd-run --unit=stroppy-ydb-storage --setenv=LD_LIBRARY_PATH=/opt/ydb/lib --collect /opt/ydb/bin/ydbd server --yaml-config /etc/ydb/config.yaml --grpc-port 2135 --ic-port 19001 --mon-port 8765 --node 1",
			"  for i in $(seq 1 120); do (echo > /dev/tcp/127.0.0.1/2135) 2>/dev/null && break; sleep 1; done",
			"fi",
			"(echo > /dev/tcp/127.0.0.1/2135) 2>/dev/null || exit 1",
		}, "\n"), nil))

		// blobstorage init: assemble group 0 FROM the yaml's blob_storage_config (canonical
		// init; a DefineBox/DefineStoragePool invoke does NOT init the static group). RETRY.
		items = append(items, commandItem("bs_init", strings.Join([]string{
			"for i in $(seq 1 60); do",
			"  out=$(LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s grpc://localhost:2135 admin blobstorage config init --yaml-file /etc/ydb/config.yaml 2>&1)",
			"  echo \"$out\"",
			"  echo \"$out\" | grep -qiE 'success|already|done|OK' && exit 0",
			"  sleep 3",
			"done",
			"exit 1",
		}, "\n"), nil))

		// create the tenant database (one ssd group). RETRY; "ALREADY_EXISTS" = success.
		items = append(items, commandItem("db_create", strings.Join([]string{
			"for i in $(seq 1 30); do",
			"  out=$(LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s grpc://localhost:2135 admin database /Root/db1 create ssd:1 2>&1)",
			"  echo \"$out\"",
			"  echo \"$out\" | grep -qiE '^OK$|ALREADY_EXISTS|success' && exit 0",
			"  sleep 3",
			"done",
			"exit 1",
		}, "\n"), nil))
	}

	// COMPUTE node serving the tenant (grpc 2136). --grpc-public-host = this node's real IP:
	// the ydb SDK does endpoint discovery on connect and dials the advertised host, which
	// otherwise defaults to the unresolvable container hostname. Idempotent (skip if 2136
	// already serving). NODE 1 is strict: it is stroppy's FLOW target and hosts the tenant
	// schemeshard, so it must serve + the workload must wait for the tenant to answer DDL
	// (it boots a few seconds after the grpc port). The OTHER nodes run a best-effort extra
	// compute node off node 1's broker — they add capacity but stroppy doesn't depend on
	// them, so a slow/late join never fails the run (exit 0 regardless).
	start := []string{
		"SELFIP=$(hostname -i | awk '{print $1}')",
		"if ! (echo > /dev/tcp/127.0.0.1/2136) 2>/dev/null; then",
		"  systemctl reset-failed stroppy-ydb-db >/dev/null 2>&1 || true",
		"  systemctl is-active --quiet stroppy-ydb-db || systemd-run --unit=stroppy-ydb-db --setenv=LD_LIBRARY_PATH=/opt/ydb/lib --collect /opt/ydb/bin/ydbd server --yaml-config /etc/ydb/config.yaml --grpc-port 2136 --grpc-public-host \"$SELFIP\" --ic-port 19002 --mon-port 8766 --tenant /Root/db1 --node-broker grpc://localhost:2135",
		"  for i in $(seq 1 120); do (echo > /dev/tcp/127.0.0.1/2136) 2>/dev/null && break; sleep 1; done",
		"fi",
		"(echo > /dev/tcp/127.0.0.1/2136) 2>/dev/null || exit 1",
		"for i in $(seq 1 120); do timeout 5 env LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s grpc://localhost:2136 -d /Root/db1 discovery list >/dev/null 2>&1 && exit 0; sleep 2; done",
		"exit 0",
	}
	if !isFirst {
		// best-effort extra compute node: broker off node 1, never fail the run.
		start = []string{
			"SELFIP=$(hostname -i | awk '{print $1}')",
			"systemctl reset-failed stroppy-ydb-db >/dev/null 2>&1 || true",
			"systemctl is-active --quiet stroppy-ydb-db || systemd-run --unit=stroppy-ydb-db --setenv=LD_LIBRARY_PATH=/opt/ydb/lib --collect /opt/ydb/bin/ydbd server --yaml-config /etc/ydb/config.yaml --grpc-port 2136 --grpc-public-host \"$SELFIP\" --ic-port 19002 --mon-port 8766 --tenant /Root/db1 --node-broker grpc://" + node1Tok + ":2135 || true",
			"exit 0",
		}
	}
	items = append(items, commandItem("start_database", strings.Join(start, "\n"), nil))
	return &renderpb.Config{Id: "ydb", Items: items}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isMultiDB reports whether the topology has more than one DATABASE component
// (i.e. a replicated/clustered shape rather than single).
func isMultiDB(topo *domain.Topology) bool {
	return len(componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE)) > 1
}

func commandItem(id, script string, bindings []*renderpb.Config_Binding) *renderpb.Config_Item {
	return &renderpb.Config_Item{
		Id:       id,
		Bindings: bindings,
		Rendered: &renderpb.Config_Item_Command{Command: &system.Cmd_Spec{
			Command: &system.Cmd_Spec_Script{Script: &system.Cmd_Script{Text: script, Shell: "/bin/bash"}},
		}},
	}
}

// ─── mysql / mariadb replication ──────────────────────────────────────────────

func renderMySQLComponent(c *domain.Topology_Component, db *domain.Database, topo *domain.Topology, totalMemoryMB int) *renderpb.Config {
	dbs := componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE)
	serverID := 1
	for i, id := range dbs {
		if id == c.GetId() {
			serverID = i + 1
		}
	}
	// MariaDB and MySQL diverge on GTID syntax: gtid_mode/enforce_gtid_consistency
	// + CHANGE REPLICATION SOURCE/SOURCE_AUTO_POSITION are MySQL-8 only — MariaDB
	// rejects gtid_mode as an unknown variable (server won't start) and uses CHANGE
	// MASTER ... MASTER_USE_GTID=slave_pos instead.
	isMaria := db.GetKind() == domain.Database_KIND_MARIADB

	conf := baseMysqld()
	conf["server-id"] = strconv.Itoa(serverID)
	conf["log_bin"] = "mysql-bin"
	if isMaria {
		// MariaDB has GTID always available; no gtid_mode toggle. log_slave_updates
		// cascades binlog events through replicas. gtid_strict_mode is intentionally
		// OFF: the replica writes local GTIDs (workload-user DDL) before replication
		// starts, and strict mode aborts the SQL thread on the resulting out-of-order
		// GTID ("would create an out-of-order sequence number").
		conf["log_slave_updates"] = "ON"
	} else {
		conf["gtid_mode"] = "ON"
		conf["enforce_gtid_consistency"] = "ON"
		conf["log_replica_updates"] = "ON"
	}

	items := []*renderpb.Config_Item{
		fileItem("my.cnf", mysqlConfigPath(db.GetKind()), formatMysqld(conf, totalMemoryMB)),
		mysqlWorkloadUserItem(),
	}

	switch ComponentRole(c) {
	case RoleReplica:
		// This node is a replica: point it at the primary's ip (late binding). The
		// primary is the RolePrimary DATABASE component — read from the topology, not
		// from a replication edge.
		primary := primaryDatabaseID(topo)
		tok := "__PRIMARY_IP__"
		// STOP first (idempotent across agent-lease re-delivery: CHANGE fails if the
		// replica is already running), then retry the whole setup — the primary's
		// `repl` user (create_replica_user) has no cross-machine ordering edge, so the
		// first attempts can hit "access denied" until the primary grants it.
		var inner string
		if isMaria {
			inner = `STOP SLAVE; CHANGE MASTER TO MASTER_HOST='` + tok +
				`', MASTER_USER='repl', MASTER_PASSWORD='repl', MASTER_USE_GTID=slave_pos; START SLAVE;`
		} else {
			inner = `STOP REPLICA; CHANGE REPLICATION SOURCE TO SOURCE_HOST='` + tok +
				`', SOURCE_USER='repl', SOURCE_PASSWORD='repl', SOURCE_AUTO_POSITION=1, GET_SOURCE_PUBLIC_KEY=1; START REPLICA;`
		}
		sql := `for i in $(seq 1 40); do mysql -e "` + inner + `" && exit 0; sleep 5; done; exit 1`
		items = append(items, commandItem("setup_replica", sql,
			[]*renderpb.Config_Binding{{Token: tok, ComponentIds: []string{primary}, Attr: AttrPrivateIP}}))
	case RolePrimary:
		// This node is the primary of a replica: provision the replication user. Use
		// the server default auth plugin (caching_sha2_password) — mysql_native_password
		// is disabled by default in 8.4; the replica pairs it with GET_SOURCE_PUBLIC_KEY.
		items = append(items, commandItem("create_replica_user",
			`mysql -e "CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY 'repl'; `+
				`GRANT REPLICATION SLAVE ON *.* TO 'repl'@'%'; FLUSH PRIVILEGES;"`, nil))
	}
	return &renderpb.Config{Id: "mysql", Items: items}
}

func renderCockroachComponent(c *domain.Topology_Component, topo *domain.Topology, totalMemoryMB int) *renderpb.Config {
	cacheMB := totalMemoryMB / 4
	if cacheMB <= 0 {
		cacheMB = 256
	}
	sqlMB := totalMemoryMB / 4
	if sqlMB <= 0 {
		sqlMB = 256
	}

	bindings := []*renderpb.Config_Binding{
		{Token: selfIPToken, ComponentIds: []string{c.GetId()}, Attr: AttrPrivateIP},
	}
	items := []*renderpb.Config_Item{}
	joinHosts := cockroachJoinHosts(c, topo, &bindings)

	// The RolePrimary node is the cluster seed: it also runs the one-shot init.
	if ComponentRole(c) == RolePrimary {
		items = append(items, commandItem("start_cockroach", renderCockroachStartScript(cacheMB, sqlMB, joinHosts), bindings))
		items = append(items, commandItem("init_cockroach", renderCockroachInitScript(), nil))
		return &renderpb.Config{Id: "cockroach", Items: items}
	}

	items = append(items, commandItem("start_cockroach", renderCockroachStartScript(cacheMB, sqlMB, joinHosts), bindings))
	return &renderpb.Config{Id: "cockroach", Items: items}
}

func renderCockroachStartScript(cacheMB, sqlMB int, joinHosts string) string {
	return strings.Join([]string{
		"set -e",
		"groupadd -f cockroach",
		"(id -u cockroach >/dev/null 2>&1 || useradd cockroach -g cockroach -d /var/lib/cockroach -m)",
		"mkdir -p /var/lib/cockroach",
		"chown -R cockroach:cockroach /var/lib/cockroach",
		// Run cockroach under systemd (transient unit), NOT `nohup &`: a backgrounded
		// process is reaped when the agent command's process tree / 60s lease ends, so
		// it never stays up. A systemd unit survives the command, the lease, and
		// restarts (same model as patroni). Idempotent: skip if already up/active.
		"if (echo > /dev/tcp/127.0.0.1/26257) 2>/dev/null; then exit 0; fi",
		"systemctl reset-failed stroppy-cockroach >/dev/null 2>&1 || true",
		fmt.Sprintf("systemctl is-active --quiet stroppy-cockroach || systemd-run --unit=stroppy-cockroach --uid=cockroach --gid=cockroach /usr/local/bin/cockroach start --insecure --advertise-addr=%s:26257 --listen-addr=0.0.0.0:26257 --sql-addr=0.0.0.0:5432 --http-addr=0.0.0.0:8080 --store=/var/lib/cockroach --cache=%dMiB --max-sql-memory=%dMiB --join=%s", selfIPToken, cacheMB, sqlMB, joinHosts),
		"for i in $(seq 1 180); do (echo > /dev/tcp/127.0.0.1/26257) 2>/dev/null && exit 0; sleep 1; done",
		"exit 1",
	}, "\n")
}

func renderCockroachInitScript() string {
	// Succeed on a fresh init ("Cluster successfully initialized") AND on a re-run
	// ("already been initialized" — idempotent across agent-lease re-delivery). The
	// old `exit ${PIPESTATUS[0]}` after the grep returned grep's own exit (1 on no
	// match), so a successful first init wrongly failed the node.
	return `/usr/local/bin/cockroach init --insecure --host=127.0.0.1:26257 2>&1 | tee /tmp/crdb-init.log || true
grep -qE "successfully initialized|already been initialized" /tmp/crdb-init.log && exit 0
exit 1`
}

// cockroachJoinHosts builds the --join target list for a cockroach node from component
// ROLES (not edges): a RoleReplica (joiner) joins the RolePrimary seed; the seed joins
// every other DATABASE node. A lone node joins itself.
func cockroachJoinHosts(c *domain.Topology_Component, topo *domain.Topology, bindings *[]*renderpb.Config_Binding) string {
	if ComponentRole(c) == RoleReplica {
		primary := primaryDatabaseID(topo)
		tok := peerToken("CRDB", primary)
		*bindings = append(*bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{primary}, Attr: AttrPrivateIP})
		return tok + ":26257"
	}

	var joins []string
	for _, dbID := range componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE) {
		if dbID == c.GetId() {
			continue
		}
		tok := peerToken("CRDB", dbID)
		*bindings = append(*bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{dbID}, Attr: AttrPrivateIP})
		joins = append(joins, tok+":26257")
	}
	if len(joins) == 0 {
		joins = append(joins, selfIPToken+":26257")
	}
	return strings.Join(joins, ",")
}

// ─── picodata cluster ─────────────────────────────────────────────────────────

func renderPicodataComponent(c *domain.Topology_Component, topo *domain.Topology, totalMemoryMB int) *renderpb.Config {
	bindings := []*renderpb.Config_Binding{{Token: selfIPToken, ComponentIds: []string{c.GetId()}, Attr: AttrPrivateIP}}
	var peers []string
	for _, pid := range componentIDsByKind(topo, domain.Topology_Component_KIND_DATABASE) {
		if pid == c.GetId() {
			peers = append(peers, selfIPToken+":3301")
			continue
		}
		tok := peerToken("PD", pid)
		peers = append(peers, tok+":3301")
		bindings = append(bindings, &renderpb.Config_Binding{Token: tok, ComponentIds: []string{pid}, Attr: AttrPrivateIP})
	}
	memMB := max(totalMemoryMB/2, 256)
	var b strings.Builder
	b.WriteString("# Generated by stroppy\ncluster:\n  name: stroppy-picodata\n")
	b.WriteString("instance:\n")
	fmt.Fprintf(&b, "  memtx:\n    memory: %dM\n", memMB)
	b.WriteString("  iproto:\n    listen: 0.0.0.0:3301\n")
	fmt.Fprintf(&b, "    advertise: %s:3301\n", selfIPToken)
	b.WriteString("  peer:\n")
	for _, p := range peers {
		fmt.Fprintf(&b, "    - %s\n", p)
	}
	// pg.advertise MUST be the real node IP: stroppy's picodata driver does cluster
	// discovery (getTopology) and dials each instance's ADVERTISED pg address. With no
	// pg.advertise it defaults to the listen addr 0.0.0.0 -> the cross-host workload
	// dials "0.0.0.0:5432" -> connection refused. (single-node renderPicodataYAML sets
	// this too; the cluster path had omitted it.)
	b.WriteString("  pg:\n    listen: 0.0.0.0:5432\n")
	fmt.Fprintf(&b, "    advertise: %s:5432\n", selfIPToken)
	b.WriteString("  http:\n    listen: 0.0.0.0:8081\n")
	return &renderpb.Config{Id: "picodata", Items: []*renderpb.Config_Item{
		fileItemBound("picodata.yaml", "/etc/picodata/picodata.yaml", b.String(), bindings),
	}}
}
