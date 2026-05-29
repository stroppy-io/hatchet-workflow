package mariadb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

const (
	groupInnoDB      = "InnoDB"
	groupConnections = "Connections"
	groupDurability  = "Durability & binlog"
	groupAdvanced    = "Advanced"
)

// sizeField is a my.cnf size/value knob. Stored as a string so it can hold a unit
// ("16G"), a percent ("25%"), or a ${var} placeholder resolved downstream (e.g.
// against the machine RAM at deploy time).
func sizeField(name, def, group, title string) *schemapb.StrB {
	return schemapb.Str(name).Default(def).Group(group).Title(title)
}

// tuningSection is the my.cnf surface (pure DB), kept aligned with the MySQL
// schema where the knobs are shared. MariaDB differs from MySQL 8.x here: it has
// NO innodb_redo_log_capacity (still innodb_log_file_size + innodb_log_files_in_group),
// and gains a native thread_pool_size.
func tuningSection() schemapb.FieldDef {
	return schemapb.Object(FieldTuning,
		// --- InnoDB ---
		sizeField(FieldInnodbBufferPoolSize, "128M", groupInnoDB, "innodb_buffer_pool_size").
			Desc("e.g. 25%-75% RAM or a ${var}; resolved downstream."),
		schemapb.Int32(FieldInnodbBufferPoolInst).Gte(1).Lte(64).Default(1).Group(groupInnoDB).
			Title("innodb_buffer_pool_instances").
			Desc("Shard the buffer pool to cut mutex contention on big pools."),
		sizeField(FieldInnodbLogFileSize, "1G", groupInnoDB, "innodb_log_file_size").
			Desc("MariaDB redo log file size (no dynamic redo capacity like MySQL 8.0.30+)."),
		schemapb.Int32(FieldInnodbLogFiles).Gte(1).Lte(100).Default(1).Group(groupInnoDB).
			Title("innodb_log_files_in_group"),
		utils.StrEnum(FieldInnodbFlushMethod, FlushMethodValues...).Default(FlushODirect).
			Group(groupInnoDB).Title("innodb_flush_method").
			Desc("O_DIRECT avoids double-buffering with the OS page cache."),
		schemapb.Int32(FieldInnodbFlushLogAtTrxComm).In(0, 1, 2).Default(1).Group(groupInnoDB).
			Title("innodb_flush_log_at_trx_commit").
			Desc("1 = full ACID durability (recommended even with Galera, where another "+
				"node can re-supply lost commits); 2/0 trade durability for throughput (bench-only)."),
		schemapb.Int32(FieldInnodbIoCapacity).Gte(100).Default(2000).Group(groupInnoDB).
			Title("innodb_io_capacity").Desc("Baseline background IOPS; ~SSD: 2000+."),
		schemapb.Int32(FieldInnodbIoCapacityMax).Gte(100).Default(4000).Group(groupInnoDB).
			Title("innodb_io_capacity_max").Desc("Burst background IOPS ceiling."),
		schemapb.Int32(FieldInnodbReadIoThreads).Gte(1).Lte(64).Default(8).Group(groupInnoDB).
			Title("innodb_read_io_threads"),
		schemapb.Int32(FieldInnodbWriteIoThreads).Gte(1).Lte(64).Default(8).Group(groupInnoDB).
			Title("innodb_write_io_threads"),
		schemapb.Int32(FieldInnodbAutoincLockMode).In(0, 1, 2).Default(2).Group(groupInnoDB).
			Title("innodb_autoinc_lock_mode").
			Desc("2 (interleaved) is fastest and REQUIRED for Galera (with ROW binlog)."),
		schemapb.Bool(FieldInnodbDoublewrite).Default(true).Group(groupInnoDB).
			Title("innodb_doublewrite").Desc("Protects against torn pages; must stay ON for Galera."),

		// --- Connections ---
		schemapb.Int32(FieldMaxConns).Gte(1).Lte(100000).Default(151).Group(groupConnections).
			Title("max_connections"),
		schemapb.Int32(FieldThreadCacheSize).Gte(0).Default(64).Group(groupConnections).
			Title("thread_cache_size"),
		schemapb.Int32(FieldThreadPoolSize).Gte(0).Default(0).Group(groupConnections).
			Title("thread_pool_size").Desc("MariaDB thread pool size; 0 = auto (CPU count)."),
		schemapb.Int32(FieldTableOpenCache).Gte(1).Default(4000).Group(groupConnections).
			Title("table_open_cache"),
		schemapb.Int32(FieldTableDefCache).Gte(400).Default(2000).Group(groupConnections).
			Title("table_definition_cache"),
		sizeField(FieldThreadStack, "292K", groupConnections, "thread_stack"),
		sizeField(FieldMaxAllowedPacket, "64M", groupConnections, "max_allowed_packet"),
		sizeField(FieldTmpTableSize, "16M", groupConnections, "tmp_table_size"),

		// --- Durability & binlog ---
		schemapb.Int32(FieldSyncBinlog).Gte(0).Default(1).Group(groupDurability).
			Title("sync_binlog").Desc("1 = flush binlog every commit (full durability)."),
		schemapb.Int32(FieldBinlogExpireSeconds).Gte(0).Default(7).Unit("days").
			Group(groupDurability).Title("expire_logs_days").
			Desc("MariaDB uses expire_logs_days (days), not binlog_expire_logs_seconds."),
		utils.StrEnum(FieldBinlogRowImage, []string{"full", "minimal", "noblob"}...).
			Default("full").Group(groupDurability).Title("binlog_row_image").
			Desc("minimal shrinks ROW binlog but needs PK on every table."),
		sizeField(FieldBinlogCacheSize, "32K", groupDurability, "binlog_cache_size"),
		sizeField(FieldMaxBinlogSize, "1G", groupDurability, "max_binlog_size"),

		// --- Advanced escape hatch (schemapb has no map kind) ---
		schemapb.List(FieldExtraParams,
			schemapb.Object(FieldParam,
				schemapb.Str(FieldKey).Required().MinLen(1),
				schemapb.Str(FieldValue).Required(),
			),
		).Group(groupAdvanced).Title("Extra parameters").
			Desc("Raw my.cnf key/value overrides (values may use ${var})."),
	).Title("Server tuning")
}
