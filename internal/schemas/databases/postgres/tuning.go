package postgres

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

const (
	groupMemory      = "Memory"
	groupWAL         = "WAL & durability"
	groupCheckpoints = "Checkpoints & bgwriter"
	groupParallelism = "Parallelism"
	groupPlanner     = "Planner & I/O"
	groupAutovacuum  = "Autovacuum"
	groupLogging     = "Logging"
	groupAdvanced    = "Advanced"
)

// sizeField is a postgresql.conf size/value knob. Stored as a string so it can
// hold a unit ("16GB"), a percent ("25%"), or a ${var} placeholder resolved
// downstream (e.g. against the machine RAM at deploy time).
func sizeField(name, def, group, title string) *schemapb.StrB {
	return schemapb.Str(name).Default(def).Group(group).Title(title)
}

// tuningSection is the full postgresql.conf surface (pure DB). Fields are flat
// inside one object and bucketed for the UI by Group() (schemapb has no nested
// sections). Sizes are strings (units / percents / ${var}); numerics are typed.
func tuningSection() schemapb.FieldDef {
	return schemapb.Object(FieldTuning,
		// --- Memory ---
		sizeField(FieldSharedBuffers, "128MB", groupMemory, "shared_buffers").
			Desc("e.g. 25% RAM or a ${var}; resolved downstream."),
		sizeField(FieldWorkMem, "4MB", groupMemory, "work_mem"),
		sizeField(FieldMaintenanceWorkMem, "64MB", groupMemory, "maintenance_work_mem"),
		sizeField(FieldEffectiveCacheSize, "4GB", groupMemory, "effective_cache_size"),
		sizeField(FieldWalBuffers, "-1", groupMemory, "wal_buffers").Desc("-1 = auto."),
		sizeField(FieldTempBuffers, "8MB", groupMemory, "temp_buffers"),
		schemapb.Double(FieldHashMemMultiplier).Gte(1).Default(2.0).Group(groupMemory).
			Title("hash_mem_multiplier"),
		utils.StrEnum(FieldHugePages, HugePagesValues...).Default(HugePagesTry).Group(groupMemory).
			Title("huge_pages"),

		// --- WAL & durability ---
		sizeField(FieldMaxWalSize, "1GB", groupWAL, "max_wal_size"),
		sizeField(FieldMinWalSize, "80MB", groupWAL, "min_wal_size"),
		utils.StrEnum(FieldWalCompression, WalCompressionValues...).Default(WalCompressionOff).
			Group(groupWAL).Title("wal_compression").Desc("lz4/zstd need PG15+ (gated at cluster)."),
		schemapb.Bool(FieldFullPageWrites).Default(true).Group(groupWAL).Title("full_page_writes"),
		schemapb.Bool(FieldFsync).Default(true).Group(groupWAL).Title("fsync").
			Desc("off = no durability — bench-only."),
		schemapb.Int32(FieldWalWriterDelay).Gte(1).Default(200).Unit("ms").Group(groupWAL).
			Title("wal_writer_delay"),
		schemapb.Bool(FieldWalLogHints).Default(false).Group(groupWAL).Title("wal_log_hints").
			Desc("Required by pg_rewind / Patroni (auto-enabled at cluster for patroni_ha)."),
		schemapb.Int32(FieldCommitDelay).Gte(0).Default(0).Unit("us").Group(groupWAL).
			Title("commit_delay"),
		schemapb.Int32(FieldCommitSiblings).Gte(0).Default(5).Group(groupWAL).Title("commit_siblings"),
		utils.StrEnum(FieldArchiveMode, ArchiveModeValues...).Default(ArchiveOff).Group(groupWAL).
			Title("archive_mode"),
		schemapb.Str(FieldArchiveCommand).Default("").Group(groupWAL).Title("archive_command"),

		// --- Checkpoints & bgwriter ---
		schemapb.Double(FieldCheckpointCompletionTarget).Gte(0).Lte(1).Default(0.9).
			Group(groupCheckpoints).Title("checkpoint_completion_target"),
		sizeField(FieldCheckpointTimeout, "5min", groupCheckpoints, "checkpoint_timeout"),
		sizeField(FieldCheckpointFlushAfter, "256kB", groupCheckpoints, "checkpoint_flush_after"),
		schemapb.Int32(FieldBgwriterDelay).Gte(10).Default(200).Unit("ms").Group(groupCheckpoints).
			Title("bgwriter_delay"),
		schemapb.Int32(FieldBgwriterLruMaxpages).Gte(0).Default(100).Group(groupCheckpoints).
			Title("bgwriter_lru_maxpages"),
		schemapb.Double(FieldBgwriterLruMultiplier).Gte(0).Default(2.0).Group(groupCheckpoints).
			Title("bgwriter_lru_multiplier"),

		// --- Parallelism ---
		schemapb.Int32(FieldMaxConnections).Gte(1).Lte(10000).Default(100).Group(groupParallelism).
			Title("max_connections"),
		schemapb.Int32(FieldMaxWorkerProcesses).Gte(0).Default(8).Group(groupParallelism).
			Title("max_worker_processes"),
		schemapb.Int32(FieldMaxParallelWorkers).Gte(0).Default(8).Group(groupParallelism).
			Title("max_parallel_workers"),
		schemapb.Int32(FieldMaxParallelWorkersPerGather).Gte(0).Default(2).Group(groupParallelism).
			Title("max_parallel_workers_per_gather"),
		schemapb.Int32(FieldMaxParallelMaintenanceWorker).Gte(0).Default(2).Group(groupParallelism).
			Title("max_parallel_maintenance_workers"),

		// --- Planner & I/O ---
		schemapb.Double(FieldRandomPageCost).Gt(0).Default(4.0).Group(groupPlanner).
			Title("random_page_cost").Desc("~1.1 for SSD/NVMe."),
		schemapb.Double(FieldSeqPageCost).Gt(0).Default(1.0).Group(groupPlanner).Title("seq_page_cost"),
		schemapb.Int32(FieldEffectiveIoConcurrency).Gte(0).Lte(1000).Default(1).Group(groupPlanner).
			Title("effective_io_concurrency").Desc("200+ for SSD."),
		schemapb.Int32(FieldMaintenanceIoConcurrenc).Gte(0).Lte(1000).Default(10).Group(groupPlanner).
			Title("maintenance_io_concurrency"),
		schemapb.Int32(FieldDefaultStatisticsTarget).Gte(1).Lte(10000).Default(100).Group(groupPlanner).
			Title("default_statistics_target"),
		schemapb.Bool(FieldJit).Default(true).Group(groupPlanner).Title("jit"),
		schemapb.Double(FieldJitAboveCost).Gte(-1).Default(100000).Group(groupPlanner).
			Title("jit_above_cost"),

		// --- Autovacuum ---
		schemapb.Bool(FieldAutovacuum).Default(true).Group(groupAutovacuum).Title("autovacuum"),
		schemapb.Int32(FieldAutovacuumMaxWorkers).Gte(1).Default(3).Group(groupAutovacuum).
			Title("autovacuum_max_workers"),
		schemapb.Int32(FieldAutovacuumNaptime).Gte(1).Default(60).Unit("s").Group(groupAutovacuum).
			Title("autovacuum_naptime"),
		schemapb.Double(FieldAutovacuumVacuumScaleFactor).Gte(0).Lte(1).Default(0.2).
			Group(groupAutovacuum).Title("autovacuum_vacuum_scale_factor"),
		schemapb.Double(FieldAutovacuumAnalyzeScaleFactor).Gte(0).Lte(1).Default(0.1).
			Group(groupAutovacuum).Title("autovacuum_analyze_scale_factor"),
		schemapb.Int32(FieldAutovacuumVacuumCostLimit).Gte(-1).Default(-1).Group(groupAutovacuum).
			Title("autovacuum_vacuum_cost_limit"),
		schemapb.Int32(FieldAutovacuumVacuumCostDelay).Gte(-1).Default(2).Unit("ms").
			Group(groupAutovacuum).Title("autovacuum_vacuum_cost_delay"),

		// --- Logging / observability ---
		schemapb.Int32(FieldLogMinDurationStatement).Gte(-1).Default(-1).Unit("ms").Group(groupLogging).
			Title("log_min_duration_statement"),
		schemapb.Bool(FieldLogCheckpoints).Default(true).Group(groupLogging).Title("log_checkpoints"),
		schemapb.Bool(FieldTrackIoTiming).Default(false).Group(groupLogging).Title("track_io_timing"),
		utils.StrEnum(FieldTrackFunctions, TrackFunctionsValues...).Default(TrackFuncNone).
			Group(groupLogging).Title("track_functions"),
		schemapb.Int32(FieldTrackActivityQuerySize).Gte(100).Default(1024).Unit("B").
			Group(groupLogging).Title("track_activity_query_size"),
		utils.StrEnum(FieldComputeQueryID, ComputeQueryIDValues...).Default(ComputeQueryIDAuto).
			Group(groupLogging).Title("compute_query_id").Desc("Needed by pg_stat_statements (PG14+)."),

		// --- Advanced escape hatch (schemapb has no map kind) ---
		schemapb.List(FieldExtraParams,
			schemapb.Object(FieldParam,
				schemapb.Str(FieldKey).Required().MinLen(1),
				schemapb.Str(FieldValue).Required(),
			),
		).Group(groupAdvanced).Title("Extra parameters").
			Desc("Raw postgresql.conf key/value overrides (values may use ${var})."),
	).Title("Server tuning")
}
