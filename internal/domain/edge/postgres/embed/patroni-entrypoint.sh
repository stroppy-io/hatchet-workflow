#!/bin/bash
set -e

# Generate patroni.yml from environment variables
cat > /tmp/patroni.yml <<EOF
scope: ${PATRONI_SCOPE:-pg}
name: ${PATRONI_NAME:-node1}

restapi:
  listen: ${PATRONI_RESTAPI_LISTEN:-0.0.0.0:8008}
  connect_address: ${PATRONI_RESTAPI_CONNECT_ADDRESS:-$(hostname):8008}

etcd3:
  hosts: ${PATRONI_ETCD3_HOSTS:-etcd-1:2379}

bootstrap:
  dcs:
    ttl: ${PATRONI_TTL:-30}
    loop_wait: ${PATRONI_LOOP_WAIT:-10}
    retry_timeout: ${PATRONI_RETRY_TIMEOUT:-10}
    maximum_lag_on_failover: ${PATRONI_MAXIMUM_LAG_ON_FAILOVER:-1048576}
    synchronous_mode: ${PATRONI_SYNCHRONOUS_MODE:-false}
    synchronous_node_count: ${PATRONI_SYNCHRONOUS_NODE_COUNT:-1}
    postgresql:
      use_pg_rewind: ${PATRONI_POSTGRESQL_USE_PG_REWIND:-true}
      parameters:
        max_connections: 100
        max_wal_senders: 10
        wal_level: replica
        hot_standby: "on"
        wal_log_hints: "on"
  initdb:
    - encoding: UTF8
    - data-checksums
  pg_hba:
    - host replication ${PATRONI_REPLICATION_USERNAME:-replicator} 0.0.0.0/0 md5
    - host all all 0.0.0.0/0 md5

postgresql:
  listen: ${PATRONI_POSTGRESQL_LISTEN:-0.0.0.0:5432}
  connect_address: ${PATRONI_POSTGRESQL_CONNECT_ADDRESS:-$(hostname):5432}
  data_dir: ${PATRONI_POSTGRESQL_DATA_DIR:-/var/lib/postgresql/data/patroni}
  pgpass: /tmp/pgpass
  authentication:
    superuser:
      username: ${PATRONI_SUPERUSER_USERNAME:-postgres}
      password: ${PATRONI_SUPERUSER_PASSWORD:-postgres}
    replication:
      username: ${PATRONI_REPLICATION_USERNAME:-replicator}
      password: ${PATRONI_REPLICATION_PASSWORD:-replicator}
  parameters:
    unix_socket_directories: '/var/run/postgresql'
EOF

exec patroni /tmp/patroni.yml
