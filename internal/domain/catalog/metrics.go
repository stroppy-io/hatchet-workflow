package catalog

// metrics is the v0 metric catalog: what a run's Result carries (keys of
// spec.result.run@1 summary/metrics as the pipeline's summary parser names
// them) and the DB/host metrics the dashboards read.
func metrics() []Metric {
	pg := []DatabaseKind{Postgres, OrioleDB, PgNoop}
	mysql := []DatabaseKind{MySQL, MariaDB}
	return []Metric{
		{Key: "tps", Title: "Throughput", Description: "Completed transactions per second over the workload.", Unit: "tps", HigherIsBetter: true, Group: "Headline", Scope: "result", RatingEligible: true},
		{Key: "latency_p50_ms", Title: "Latency p50", Unit: "ms", Group: "Headline", Scope: "result", RatingEligible: true},
		{Key: "latency_p95_ms", Title: "Latency p95", Unit: "ms", Group: "Headline", Scope: "result", RatingEligible: true},
		{Key: "latency_p99_ms", Title: "Latency p99", Unit: "ms", Group: "Headline", Scope: "result", RatingEligible: true},
		{Key: "errors", Title: "Errors", Description: "Failed iterations and queries.", Unit: "count", Group: "Headline", Scope: "result"},
		{Key: "iterations_total", Title: "Iterations", Unit: "count", HigherIsBetter: true, Group: "Workload", Scope: "result"},
		{Key: "failed_iterations_total", Title: "Failed iterations", Unit: "count", Group: "Workload", Scope: "result"},
		{Key: "failed_queries_total", Title: "Failed queries", Unit: "count", Group: "Workload", Scope: "result"},
		{Key: "iteration_duration_avg", Title: "Iteration duration, avg", Unit: "ms", Group: "Workload", Scope: "result"},
		{Key: "iteration_duration_p90", Title: "Iteration duration, p90", Unit: "ms", Group: "Workload", Scope: "result"},
		{Key: "iteration_duration_p99", Title: "Iteration duration, p99", Unit: "ms", Group: "Workload", Scope: "result"},
		{Key: "load_duration_seconds", Title: "Data load duration", Unit: "s", Group: "Bootstrap", Scope: "result"},

		{Key: "node_cpu_usage", Title: "CPU usage", Unit: "%", Group: "Host", Scope: "host"},
		{Key: "node_memory_used_bytes", Title: "Memory used", Unit: "bytes", Group: "Host", Scope: "host"},
		{Key: "node_disk_io_bytes", Title: "Disk I/O", Unit: "bytes/s", Group: "Host", Scope: "host"},
		{Key: "node_network_bytes", Title: "Network", Unit: "bytes/s", Group: "Host", Scope: "host"},

		{Key: "pg_stat_database_xact_commit", Title: "Commits", Unit: "1/s", HigherIsBetter: true, Group: "PostgreSQL", Scope: "db", DBKinds: pg},
		{Key: "pg_stat_database_xact_rollback", Title: "Rollbacks", Unit: "1/s", Group: "PostgreSQL", Scope: "db", DBKinds: pg},
		{Key: "pg_stat_database_blks_hit_ratio", Title: "Buffer cache hit ratio", Unit: "%", HigherIsBetter: true, Group: "PostgreSQL", Scope: "db", DBKinds: pg},
		{Key: "pg_stat_activity_count", Title: "Backends", Unit: "count", Group: "PostgreSQL", Scope: "db", DBKinds: pg},
		{Key: "pg_locks_count", Title: "Locks", Unit: "count", Group: "PostgreSQL", Scope: "db", DBKinds: pg},
		{Key: "pg_replication_lag_seconds", Title: "Replication lag", Unit: "s", Group: "PostgreSQL", Scope: "db", DBKinds: pg},

		{Key: "mysql_global_status_queries", Title: "Queries", Unit: "1/s", HigherIsBetter: true, Group: "MySQL", Scope: "db", DBKinds: mysql},
		{Key: "mysql_global_status_threads_connected", Title: "Threads connected", Unit: "count", Group: "MySQL", Scope: "db", DBKinds: mysql},
		{Key: "mysql_global_status_innodb_buffer_pool_hit_ratio", Title: "InnoDB buffer pool hit ratio", Unit: "%", HigherIsBetter: true, Group: "MySQL", Scope: "db", DBKinds: mysql},
		{Key: "mysql_slave_status_seconds_behind_master", Title: "Replica lag", Unit: "s", Group: "MySQL", Scope: "db", DBKinds: mysql},

		{Key: "ydb_table_query_rate", Title: "Query rate", Unit: "1/s", HigherIsBetter: true, Group: "YDB", Scope: "db", DBKinds: []DatabaseKind{YDB, YDBManaged}},
		{Key: "ydb_table_query_latency_p99", Title: "Query latency p99", Unit: "ms", Group: "YDB", Scope: "db", DBKinds: []DatabaseKind{YDB, YDBManaged}},
		{Key: "cockroach_sql_txn_commit_count", Title: "Transactions committed", Unit: "1/s", HigherIsBetter: true, Group: "CockroachDB", Scope: "db", DBKinds: []DatabaseKind{Cockroach}},
		{Key: "cockroach_sql_service_latency_p99", Title: "SQL latency p99", Unit: "ms", Group: "CockroachDB", Scope: "db", DBKinds: []DatabaseKind{Cockroach}},
		{Key: "picodata_sql_query_total", Title: "SQL queries", Unit: "1/s", HigherIsBetter: true, Group: "Picodata", Scope: "db", DBKinds: []DatabaseKind{Picodata}},
	}
}
