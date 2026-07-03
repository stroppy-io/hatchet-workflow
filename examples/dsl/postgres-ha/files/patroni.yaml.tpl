scope: postgres-ha
namespace: /stroppy/
name: "{{ .NodeName }}"

restapi:
  listen: 0.0.0.0:8008
  connect_address: "{{ .NodeIP }}:8008"

etcd3:
  hosts:
    - db:2379

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    synchronous_mode: false
    postgresql:
      use_pg_rewind: true
      parameters:
        wal_level: replica
        hot_standby: "on"
        max_wal_senders: 10
        max_replication_slots: 10
  initdb:
    - encoding: UTF8
    - data-checksums

postgresql:
  listen: 0.0.0.0:5432
  connect_address: "{{ .NodeIP }}:5432"
  data_dir: /home/postgres/pgdata
  bin_dir: /usr/lib/postgresql/16/bin
  authentication:
    replication:
      username: replicator
      password: "{{ .ReplicationPassword }}"
    superuser:
      username: postgres
      password: "{{ .SuperuserPassword }}"

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
