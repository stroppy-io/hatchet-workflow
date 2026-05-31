package recipe

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
)

// ydbRecipe is the YDB (self-hosted) deploy engine. Single-node combined path
// (storage + dynamic on one box). Multi-node mirror-3dc is a TODO.
type ydbRecipe struct{}

const (
	ydbVersion  = "25.2.1.24"
	ydbGRPCPort = 2136
	ydbDatabase = "/Root/testdb"
)

func (ydbRecipe) BuildComponent(string, map[string]any, map[string]any, Refs) *topology.Component_Strategy {
	return nil
}
func (ydbRecipe) Wire(map[string][]string) []*topology.Connection { return nil }

func (ydbRecipe) Steps(self Node, cl Cluster, db, wl map[string]any, refs Refs) []Step {
	switch self.Role {
	case expand.RoleDatabase:
		return ydbNodeSteps(self)
	case expand.RoleWorkload:
		host := "127.0.0.1"
		if n, ok := cl.First(expand.RoleDatabase); ok {
			host = n.IP
		}
		dsn := fmt.Sprintf("grpc://%s:%d%s", host, ydbGRPCPort, ydbDatabase)
		return stroppyRunSteps(driverYDB, "tpcc/tx", dsn, refs)
	default:
		return nil
	}
}

// ydbNodeSteps is the minimal single-node (combined) YDB: install ydbd, render a
// 1-host erasure=none static config, init blobstorage + create the database, then
// start the dynamic node. Self host is the node's own hostname (container name).
func ydbNodeSteps(self Node) []Step {
	install := fmt.Sprintf(`set -e
if [ ! -x /opt/ydb/bin/ydbd ]; then
  mkdir -p /opt/ydb
  curl -fSL --retry 5 --retry-delay 3 "https://binaries.ydb.tech/release/%[1]s/ydbd-%[1]s-linux-amd64.tar.gz" | tar -xz --strip-component=1 -C /opt/ydb
fi
groupadd -f ydb && (id -u ydb >/dev/null 2>&1 || useradd ydb -g ydb)
mkdir -p /opt/ydb/cfg /ydb_data && chown -R ydb:ydb /ydb_data`, ydbVersion)

	config := `static_erasure: none
host_configs:
- host_config_id: 1
  drive:
  - path: /ydb_data/pdisk.data
    type: SSD
hosts:
- host: __YDB_SELF__
  host_config_id: 1
  walle_location: {body: 1, data_center: 'zone-a', rack: '1'}
domains_config:
  domain:
  - name: Root
    storage_pool_types:
    - kind: ssd
      pool_config:
        box_id: 1
        erasure_species: none
        kind: ssd
        pdisk_filter:
        - property: [{type: SSD}]
        vdisk_kind: Default
  state_storage:
  - ring: {node: [1], nto_select: 1}
    ssid: 1
table_service_config:
  sql_version: 1
actor_system_config:
  use_auto_config: true
  node_type: STORAGE
  cpu_count: 2
blob_storage_config:
  service_set:
    groups:
    - erasure_species: none
      rings:
      - fail_domains:
        - vdisk_locations:
          - {node_id: 1, pdisk_category: SSD, path: /ydb_data/pdisk.data}
channel_profile_config:
  profile:
  - channel:
    - {erasure_species: none, pdisk_category: 0, storage_pool_kind: ssd}
    - {erasure_species: none, pdisk_category: 0, storage_pool_kind: ssd}
    - {erasure_species: none, pdisk_category: 0, storage_pool_kind: ssd}
grpc_config:
  port: 2136
`

	writeConfig := fmt.Sprintf(`set -e
mkdir -p /opt/ydb/cfg
SELF=$(hostname)
sed "s/__YDB_SELF__/$SELF/g" > /opt/ydb/cfg/config.yaml <<'YCFG'
%sYCFG
truncate -s 20G /ydb_data/pdisk.data
chown ydb:ydb /ydb_data/pdisk.data
LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd admin bs disk obliterate /ydb_data/pdisk.data || true`, config)

	startStorage := fmt.Sprintf(`set -e
systemctl stop ydbd-storage 2>/dev/null || true; systemctl reset-failed ydbd-storage 2>/dev/null || true
systemd-run --unit=ydbd-storage --uid=ydb --gid=ydb --setenv=LD_LIBRARY_PATH=/opt/ydb/lib \
  /opt/ydb/bin/ydbd server --yaml-config /opt/ydb/cfg/config.yaml \
  --grpc-port %d --ic-port 19001 --mon-port 8765 --node static
for i in $(seq 1 60); do (echo > /dev/tcp/localhost/%d) 2>/dev/null && exit 0; sleep 1; done
echo "ydbd-storage not listening" >&2; journalctl -u ydbd-storage --no-pager -n 50 >&2 || true; exit 1`,
		ydbGRPCPort, ydbGRPCPort)

	initCluster := fmt.Sprintf(`set -e
SELF=$(hostname)
for i in $(seq 1 30); do LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s grpc://$SELF:%[1]d admin blobstorage config init --yaml-file /opt/ydb/cfg/config.yaml 2>&1 && break; sleep 2; done
for i in $(seq 1 15); do LD_LIBRARY_PATH=/opt/ydb/lib /opt/ydb/bin/ydbd -s grpc://$SELF:%[1]d admin database %s create ssd:1 2>&1 && exit 0; sleep 2; done
exit 1`, ydbGRPCPort, ydbDatabase)

	startDatabase := fmt.Sprintf(`set -e
SELF=$(hostname)
systemctl stop ydbd-database 2>/dev/null || true; systemctl reset-failed ydbd-database 2>/dev/null || true
systemd-run --unit=ydbd-database --uid=ydb --gid=ydb --setenv=LD_LIBRARY_PATH=/opt/ydb/lib \
  /opt/ydb/bin/ydbd server --yaml-config /opt/ydb/cfg/config.yaml \
  --grpc-port %d --ic-port 19002 --mon-port 8766 --tenant %s --node-broker grpc://$SELF:%d
sleep 5`, ydbGRPCPort, ydbDatabase, ydbGRPCPort)

	return []Step{
		aptStep("apt deps", "curl ca-certificates tar"),
		cmdStep("install ydbd", install),
		cmdStep("write ydb config", writeConfig),
		cmdStep("start ydb storage", startStorage),
		cmdStep("init ydb cluster", initCluster),
		cmdStep("start ydb database", startDatabase),
	}
}
