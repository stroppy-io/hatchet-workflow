package picodata

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

const (
	groupMemory   = "Memory"
	groupStorage  = "Storage"
	groupNetwork  = "Network"
	groupProtocol = "Protocols"
	groupLogging  = "Logging"
	groupAdvanced = "Advanced"
)

// sizeField is a picodata.yaml size/value knob. Stored as a string so it can
// hold a unit ("2GB"), a percent ("25%") or a ${var} placeholder resolved
// downstream (e.g. against the machine RAM at deploy time).
func sizeField(name, def, group, title string) *schemapb.StrB {
	return schemapb.Str(name).Default(def).Group(group).Title(title)
}

// configSection is the picodata.yaml instance.* surface (pure DB). Fields are
// flat inside one object and bucketed for the UI by Group() (schemapb has no
// nested sections). Sizes are strings (units / percents / ${var}); numerics are
// typed. Ports/protocol toggles are LOGICAL — actual bind addresses (listen)
// are deployment concerns owned by the cluster schema.
func configSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldConfig,
		// --- Memory (memtx) ---
		sizeField(FieldMemtxMemory, "2GB", groupMemory, "memtx.memory").
			Desc("RAM budget for the in-memory (memtx) engine that stores tuples. "+
				"Accepts a unit (2GB), a percent of node RAM (50%) or a ${var}; resolved downstream."),
		sizeField(FieldMaxTupleSz, "1MB", groupMemory, "memtx.max_tuple_size").
			Desc("Largest single tuple memtx will store."),

		// --- Storage (vinyl on-disk LSM engine) ---
		sizeField(FieldVinylMemory, "128MB", groupStorage, "vinyl.memory").
			Desc("Write buffer / in-memory level budget for the on-disk vinyl engine."),
		sizeField(FieldVinylCache, "128MB", groupStorage, "vinyl.cache").
			Desc("Read cache size for the vinyl engine."),
		schemapb.Int32(FieldCheckpointInterval).Gte(0).Default(3600).Unit("s").
			Group(groupStorage).Title("checkpoint.interval").
			Desc("Seconds between snapshot checkpoints; 0 disables periodic checkpoints."),
		schemapb.Int32(FieldCheckpointCount).Gte(1).Default(2).
			Group(groupStorage).Title("checkpoint.count").
			Desc("Number of snapshots+xlogs to retain."),

		// --- Network (box-level transport) ---
		schemapb.Int32(FieldNetMsgMax).Gte(2).Lte(1048576).Default(1024).
			Group(groupNetwork).Title("net_msg_max").
			Desc("Max number of concurrent requests in flight before the transport applies back-pressure."),
		schemapb.Int32(FieldReadahead).Gte(128).Lte(67108864).Default(16384).Unit("B").
			Group(groupNetwork).Title("readahead").
			Desc("Per-connection read-ahead buffer for the binary protocol."),

		// --- Protocols ---
		// iproto: the binary protocol port, also used for peer/raft traffic.
		schemapb.Int32(FieldListenPort).Gte(1).Lte(65535).Default(3301).
			Group(groupProtocol).Title("iproto.listen port").
			Desc("Binary-protocol / peer port (the actual bind address is wired at the cluster layer)."),
		// pg: PostgreSQL wire protocol — how stroppy connects.
		schemapb.Bool(FieldPgEnabled).Default(true).
			Group(groupProtocol).Title("Enable PG protocol").
			Desc("Expose the PostgreSQL wire protocol (pg.listen)."),
		schemapb.Int32(FieldPgPort).Gte(1).Lte(65535).Default(5432).
			When(utils.IsTrue(rp(pfx, FieldConfig, FieldPgEnabled))).
			Group(groupProtocol).Title("pg.listen port"),
		schemapb.Bool(FieldPgSSL).Default(false).
			When(utils.IsTrue(rp(pfx, FieldConfig, FieldPgEnabled))).
			Group(groupProtocol).Title("pg.ssl").
			Desc("Require TLS on the PG protocol (cert/key paths are wired at the cluster layer)."),
		// http: monitoring / admin endpoint.
		schemapb.Bool(FieldHTTPEnabled).Default(true).
			Group(groupProtocol).Title("Enable HTTP").
			Desc("Expose the HTTP monitoring/admin endpoint (health, metrics)."),
		schemapb.Int32(FieldHTTPPort).Gte(1).Lte(65535).Default(8081).
			When(utils.IsTrue(rp(pfx, FieldConfig, FieldHTTPEnabled))).
			Group(groupProtocol).Title("http.listen port"),

		// --- Logging ---
		utils.StrEnum(FieldLogLevel, LogLevelValues...).Default(LogInfo).
			Group(groupLogging).Title("log.level"),
		utils.StrEnum(FieldLogFormat, LogFormatValues...).Default(LogFormatPlain).
			Group(groupLogging).Title("log.format"),

		// --- Advanced escape hatch (schemapb has no map kind) ---
		schemapb.List(FieldExtraParams,
			schemapb.Object(FieldParam,
				schemapb.Str(FieldKey).Required().MinLen(1),
				schemapb.Str(FieldValue).Required(),
			),
		).Group(groupAdvanced).Title("Extra parameters").
			Desc("Raw picodata.yaml instance.* key/value overrides (values may use ${var})."),
	).Title("Instance configuration")
}
