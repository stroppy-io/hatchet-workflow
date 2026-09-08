package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Etcd3 is cfg.etcd@3 — the etcd 3.5 YAML configuration file.
//
// Keys and defaults verified against
// https://etcd.io/docs/v3.5/op-guide/configuration/ (the `doc:` markers name
// the key). Field names are the etcd keys with dashes replaced by underscores,
// because schemapb field names must be identifiers; the template writes the
// dashed form.
//
//nolint:funlen // one flat key table
func Etcd3() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("etcd", 3)).
		Descr("etcd 3.5 configuration file for the Patroni DCS.").
		Strict().Coerce().
		Fields(
			// doc: configuration — name, default 'default'
			schemapb.Str("name").Title("Member name").Group("Cluster").
				Desc("Human-readable name of this etcd member; must be unique in the cluster. Filled by the server from the node's role index.").
				Pattern(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`).Default("default"),
			// doc: configuration — data-dir, default '${name}.etcd'
			schemapb.Str("data_dir").Title("Data directory").Group("Storage").
				Desc("Path to the member's data directory (WAL and snapshots).").
				Pattern(`^/.+$`).Default("/var/lib/etcd"),

			// doc: configuration — listen-peer-urls, default http://localhost:2380
			schemapb.Str("listen_peer_urls").Title("Listen peer URLs").Group("Listen").
				Desc("Comma-separated URLs this member listens on for peer traffic.").
				MinLen(1).MaxLen(1024).Default("http://0.0.0.0:2380"),
			// doc: configuration — listen-client-urls, default http://localhost:2379
			schemapb.Str("listen_client_urls").Title("Listen client URLs").Group("Listen").
				Desc("Comma-separated URLs this member listens on for client traffic.").
				MinLen(1).MaxLen(1024).Default("http://0.0.0.0:2379"),

			// doc: configuration — initial-advertise-peer-urls
			schemapb.Str("initial_advertise_peer_urls").Title("Advertise peer URLs").Group("Cluster").
				Desc("Peer URLs the other members use to reach this one. Filled by the server from the node's address.").
				MinLen(1).MaxLen(1024).Nullable(),
			// doc: configuration — advertise-client-urls
			schemapb.Str("advertise_client_urls").Title("Advertise client URLs").Group("Cluster").
				Desc("Client URLs Patroni uses to reach this member. Filled by the server from the node's address.").
				MinLen(1).MaxLen(1024).Nullable(),
			// doc: configuration — initial-cluster, default 'default=http://localhost:2380'
			schemapb.List("initial_cluster", schemapb.Str("member").MinLen(3).MaxLen(256)).
				Title("Initial cluster").Group("Cluster").
				Desc("Bootstrap membership as `name=peerURL` entries. Filled by the server from the etcd topology.").
				MaxItems(16).Unique().Nullable(),
			schemapb.Computed("initial_cluster_rendered",
				`("initial_cluster" in root) ? root.initial_cluster.join(",") : ""`).
				Title("Rendered initial cluster").Group("Cluster").
				Desc("The initial_cluster list joined with commas — the literal value of the etcd key.").
				Result(schemapb.ResultString),

			// doc: configuration — initial-cluster-state, default 'new'
			schemapb.Choice("initial_cluster_state").Title("Initial cluster state").Group("Cluster").
				Desc("`new` bootstraps a fresh cluster, `existing` joins one that already exists.").
				Opt(schemapb.StrV("new"), "new").Opt(schemapb.StrV("existing"), "existing").
				Default(schemapb.StrV("new")),
			// doc: configuration — initial-cluster-token, default 'etcd-cluster'
			schemapb.Str("initial_cluster_token").Title("Initial cluster token").Group("Cluster").
				Desc("Token isolating this cluster from any other etcd cluster on the same network.").
				MinLen(1).MaxLen(128).Default("etcd-cluster"),

			// doc: configuration — heartbeat-interval, default 100 (ms)
			schemapb.Int64("heartbeat_interval").Title("Heartbeat interval").Group("Raft").Unit("ms").
				Desc("Interval between leader heartbeats; roughly the round-trip time between members.").
				Gte(1).Lte(60000).Default(100),
			// doc: configuration — election-timeout, default 1000 (ms)
			schemapb.Int64("election_timeout").Title("Election timeout").Group("Raft").Unit("ms").
				Desc("Time a follower waits without a heartbeat before starting an election; etcd recommends 10x the heartbeat interval.").
				Gte(10).Lte(600000).Default(1000),
			// doc: configuration — snapshot-count, default 100000
			schemapb.Int64("snapshot_count").Title("Snapshot count").Group("Storage").
				Desc("Committed transactions between snapshots to disk.").
				Gte(1).Lte(100000000).Default(100000),
			// doc: configuration — quota-backend-bytes, default 0 (2GB effective)
			schemapb.Int64("quota_backend_bytes").Title("Backend quota").Group("Storage").Unit("B").
				Desc("Alarm threshold for the backend size; 0 keeps etcd's default of 2GB.").
				Gte(0).Lte(68719476736).Default(0),
			// doc: configuration — auto-compaction-mode, default 'periodic'
			schemapb.Choice("auto_compaction_mode").Title("Auto compaction mode").Group("Storage").
				Desc("`periodic` interprets the retention as a time window, `revision` as a number of revisions.").
				Opt(schemapb.StrV("periodic"), "periodic").Opt(schemapb.StrV("revision"), "revision").
				Default(schemapb.StrV("periodic")),
			// doc: configuration — auto-compaction-retention, default '0' (disabled)
			schemapb.Str("auto_compaction_retention").Title("Auto compaction retention").Group("Storage").
				Desc("Retention for automatic compaction: '0' disables it, '1h' or '1' mean one hour in periodic mode.").
				Pattern(`^[0-9]+[hm]?$`).Default("1h"),
			// doc: configuration — max-request-bytes, default 1572864
			schemapb.Int64("max_request_bytes").Title("Max request bytes").Group("Limits").Unit("B").
				Desc("Largest client request etcd accepts.").
				Gte(1024).Lte(104857600).Default(1572864),
			// doc: configuration — enable-v2, default false in 3.5 (deprecated)
			trueFalse("enable_v2", "false").Title("Enable v2 API").Group("Limits").
				Desc("The deprecated etcd v2 API. Off: Patroni 3.x uses etcd3 exclusively."),

			customField().
				Title("Extra etcd keys").
				Desc("Raw top-level etcd YAML keys appended verbatim; keys must match "+customKeyPattern+". Entry order is not preserved."),
			schemapb.Computed("custom_rendered",
				`("custom" in root) ? root.custom.map(k, k + ": " + string(root.custom[k])).join("\n") : ""`).
				Title("Rendered extra keys").Group("Custom").
				Desc("The `custom` map joined into YAML lines appended at the end of the file.").
				Result(schemapb.ResultString),
		).
		Rules(
			customKeyRule(),
			schemapb.Rule(`int(root.election_timeout) >= 5 * int(root.heartbeat_interval)`,
				"election-timeout should be at least 5x heartbeat-interval (etcd recommends 10x)").
				ID("election-vs-heartbeat"),
		).
		Template("conf", `# etcd 3.5 configuration — generated by stroppy.
name: {{{values.name}}}
data-dir: {{{values.data_dir}}}
listen-peer-urls: {{{values.listen_peer_urls}}}
listen-client-urls: {{{values.listen_client_urls}}}
{{#values.initial_advertise_peer_urls}}initial-advertise-peer-urls: {{{.}}}
{{/values.initial_advertise_peer_urls}}{{#values.advertise_client_urls}}advertise-client-urls: {{{.}}}
{{/values.advertise_client_urls}}{{#values.initial_cluster_rendered}}initial-cluster: {{{values.initial_cluster_rendered}}}
{{/values.initial_cluster_rendered}}initial-cluster-state: {{{values.initial_cluster_state}}}
initial-cluster-token: {{{values.initial_cluster_token}}}
heartbeat-interval: {{{values.heartbeat_interval}}}
election-timeout: {{{values.election_timeout}}}
snapshot-count: {{{values.snapshot_count}}}
quota-backend-bytes: {{{values.quota_backend_bytes}}}
auto-compaction-mode: {{{values.auto_compaction_mode}}}
auto-compaction-retention: "{{{values.auto_compaction_retention}}}"
max-request-bytes: {{{values.max_request_bytes}}}
enable-v2: {{{values.enable_v2}}}
{{#values.custom_rendered}}{{{values.custom_rendered}}}
{{/values.custom_rendered}}`).
		MustBuild()
}
