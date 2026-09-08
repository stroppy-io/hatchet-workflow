package cfg

import (
	"strings"
	"testing"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

// renderDefaults resolves an empty value map (so every default is seeded) and
// renders the "conf" template — the exact file a stock deploy would write.
func renderDefaults(t *testing.T, s *schemapb.Schema) string {
	t.Helper()

	vals, res, err := s.Resolve(map[string]any{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if res.Blocking() {
		t.Fatalf("resolve errors: %v", res.GetErrors())
	}

	out, err := s.Render("conf", vals)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	return out
}

func wantLines(t *testing.T, text string, lines ...string) {
	t.Helper()

	for _, want := range lines {
		if !strings.Contains(text, want+"\n") {
			t.Errorf("rendered conf lacks %q:\n%s", want, text)
		}
	}
}

func dontWantLines(t *testing.T, text string, subs ...string) {
	t.Helper()

	for _, bad := range subs {
		if strings.Contains(text, bad) {
			t.Errorf("rendered conf unexpectedly contains %q:\n%s", bad, text)
		}
	}
}

// pgFull is the "everything set" value, cluster-filled fields included.
func pgFull() map[string]any {
	return map[string]any{
		"shared_buffers":                        int64(16384),
		"huge_pages":                            "on",
		"work_mem":                              int64(64),
		"maintenance_work_mem":                  int64(2048),
		"effective_cache_size":                  int64(49152),
		"temp_buffers":                          int64(32),
		"hash_mem_multiplier":                   2.5,
		"wal_level":                             "logical",
		"max_wal_size":                          int64(32768),
		"min_wal_size":                          int64(4096),
		"checkpoint_timeout":                    int64(900),
		"checkpoint_completion_target":          0.9,
		"wal_compression":                       "zstd",
		"synchronous_commit":                    "remote_apply",
		"wal_buffers":                           "64MB",
		"fsync":                                 "on",
		"full_page_writes":                      "on",
		"wal_writer_delay":                      int64(100),
		"max_wal_senders":                       int64(16),
		"max_replication_slots":                 int64(16),
		"hot_standby":                           "on",
		"hot_standby_feedback":                  "on",
		"wal_keep_size":                         int64(8192),
		"synchronous_standby_names":             "ANY 1 (pg-1, pg-2)",
		"seq_page_cost":                         1.0,
		"random_page_cost":                      1.1,
		"effective_io_concurrency":              int64(256),
		"jit":                                   "off",
		"default_statistics_target":             int64(500),
		"autovacuum":                            "on",
		"autovacuum_max_workers":                int64(8),
		"autovacuum_naptime":                    int64(15),
		"autovacuum_vacuum_threshold":           int64(200),
		"autovacuum_vacuum_insert_threshold":    int64(2000),
		"autovacuum_analyze_threshold":          int64(200),
		"autovacuum_vacuum_scale_factor":        0.05,
		"autovacuum_vacuum_insert_scale_factor": 0.1,
		"autovacuum_analyze_scale_factor":       0.02,
		"autovacuum_vacuum_cost_delay":          1.0,
		"autovacuum_vacuum_cost_limit":          int64(2000),
		"vacuum_cost_delay":                     0.0,
		"vacuum_cost_limit":                     int64(400),
		"vacuum_cost_page_hit":                  int64(1),
		"vacuum_cost_page_miss":                 int64(2),
		"vacuum_cost_page_dirty":                int64(20),
		"listen_addresses":                      "*",
		"port":                                  int64(5432),
		"max_connections":                       int64(500),
		"superuser_reserved_connections":        int64(5),
		"ssl":                                   "off",
		"tcp_keepalives_idle":                   int64(60),
		"tcp_keepalives_interval":               int64(10),
		"tcp_keepalives_count":                  int64(6),
		"max_worker_processes":                  int64(32),
		"max_parallel_workers":                  int64(16),
		"max_parallel_workers_per_gather":       int64(4),
		"max_parallel_maintenance_workers":      int64(4),
		"max_locks_per_transaction":             int64(256),
		"deadlock_timeout":                      int64(2000),
		"lock_timeout":                          int64(30000),
		"statement_timeout":                     int64(60000),
		"idle_in_transaction_session_timeout":   int64(120000),
		"logging_collector":                     "off",
		"log_destination":                       "stderr,jsonlog",
		"log_min_duration_statement":            int64(1000),
		"log_checkpoints":                       "on",
		"log_disconnections":                    "on",
		"log_lock_waits":                        "on",
		"log_temp_files":                        int64(0),
		"log_autovacuum_min_duration":           int64(0),
		"log_line_prefix":                       "%m [%p] %u@%d ",
		"log_statement":                         "ddl",
		"extensions":                            []any{"pg_stat_statements"},
		"pg_stat_statements_max":                int64(10000),
		"pg_stat_statements_track":              "all",
		"timezone":                              "UTC",
		"lc_messages":                           "C",
		"default_text_search_config":            "pg_catalog.simple",
		"custom":                                map[string]any{"bgwriter_delay": "10ms"},
	}
}

func pgInvalid() []schematest.Invalid {
	return []schematest.Invalid{
		{Value: map[string]any{"shared_buffers": int64(0)}, Code: "GTE_VIOLATED", Path: "shared_buffers"},
		{Value: map[string]any{"wal_level": "none"}, Code: "CHOICE_NOT_ALLOWED", Path: "wal_level"},
		{Value: map[string]any{"wal_buffers": "16"}, Code: "PATTERN_MISMATCH", Path: "wal_buffers"},
		{Value: map[string]any{"port": int64(70000)}, Code: "LTE_VIOLATED", Path: "port"},
		{Value: map[string]any{"custom": map[string]any{"Bad Key": "1"}}, Code: "RULE_VIOLATED", Path: "custom-keys"},
		{
			Value: map[string]any{"min_wal_size": int64(4096), "max_wal_size": int64(1024)},
			Code:  "RULE_VIOLATED", Path: "wal-size-order",
		},
		{Value: map[string]any{"wal_level": "minimal"}, Code: "RULE_VIOLATED", Path: "minimal-wal-no-senders"},
		{Value: map[string]any{"nope": 1}, Code: "UNKNOWN_FIELD", Path: "nope"},
	}
}

// pgContains holds only substrings true of BOTH the default and the full
// render (schematest applies Contains to every valid value).
var pgContains = []string{
	"shared_buffers = ",
	"checkpoint_timeout = ",
	"max_connections = ",
	"log_line_prefix = '",
	"fsync = on",
	"listen_addresses = '*'",
	"timezone = 'UTC'",
	"# --- Replication ---",
}

func runPG(t *testing.T, s *schemapb.Schema, extraValid map[string]any) {
	t.Helper()

	full := pgFull()
	for k, v := range extraValid {
		full[k] = v
	}

	schematest.Run(t, s, schematest.Cases{
		Valid:    []map[string]any{{}, full},
		Invalid:  pgInvalid(),
		Render:   "conf",
		Contains: pgContains,
	})
}

// pgDefaultLines are the upstream defaults every major renders identically.
var pgDefaultLines = []string{
	"shared_buffers = 128MB",
	"huge_pages = try",
	"temp_buffers = 8MB",
	"work_mem = 4MB",
	"hash_mem_multiplier = 2",
	"maintenance_work_mem = 64MB",
	"effective_cache_size = 4096MB",
	"wal_level = replica",
	"max_wal_size = 1024MB",
	"min_wal_size = 80MB",
	"checkpoint_timeout = 300s",
	"checkpoint_completion_target = 0.9",
	"wal_compression = off",
	"synchronous_commit = on",
	"wal_buffers = -1",
	"full_page_writes = on",
	"wal_writer_delay = 200ms",
	"max_wal_senders = 10",
	"max_replication_slots = 10",
	"hot_standby = on",
	"hot_standby_feedback = off",
	"wal_keep_size = 0MB",
	"seq_page_cost = 1",
	"random_page_cost = 4",
	"jit = on",
	"default_statistics_target = 100",
	"autovacuum = on",
	"autovacuum_max_workers = 3",
	"autovacuum_naptime = 60s",
	"autovacuum_vacuum_threshold = 50",
	"autovacuum_vacuum_insert_threshold = 1000",
	"autovacuum_analyze_threshold = 50",
	"autovacuum_vacuum_scale_factor = 0.2",
	"autovacuum_analyze_scale_factor = 0.1",
	"autovacuum_vacuum_cost_delay = 2ms",
	"autovacuum_vacuum_cost_limit = -1",
	"vacuum_cost_delay = 0ms",
	"vacuum_cost_limit = 200",
	"vacuum_cost_page_hit = 1",
	"vacuum_cost_page_miss = 2",
	"vacuum_cost_page_dirty = 20",
	"port = 5432",
	"max_connections = 100",
	"superuser_reserved_connections = 3",
	"ssl = off",
	"max_worker_processes = 8",
	"max_parallel_workers = 8",
	"max_parallel_workers_per_gather = 2",
	"max_locks_per_transaction = 64",
	"deadlock_timeout = 1000ms",
	"lock_timeout = 0ms",
	"statement_timeout = 0ms",
	"idle_in_transaction_session_timeout = 0ms",
	"logging_collector = off",
	"log_destination = 'stderr'",
	"log_min_duration_statement = -1",
	"log_checkpoints = on",
	"log_lock_waits = off",
	"log_temp_files = -1",
	"log_autovacuum_min_duration = 600000",
	"log_statement = none",
	"lc_messages = 'C'",
	"default_text_search_config = 'pg_catalog.simple'",
}

func TestPostgresqlConf15(t *testing.T) {
	s := PostgresqlConf15()
	runPG(t, s, nil)

	out := renderDefaults(t, s)
	wantLines(t, out, pgDefaultLines...)
	wantLines(t, out, "effective_io_concurrency = 1", "log_connections = off")
	dontWantLines(t, out, "io_method", "io_workers", "\nreserved_connections =",
		"transaction_timeout", "autovacuum_vacuum_max_threshold", "autovacuum_worker_slots",
		"shared_preload_libraries", "pg_stat_statements")
}

func TestPostgresqlConf16(t *testing.T) {
	s := PostgresqlConf16()
	runPG(t, s, map[string]any{"reserved_connections": int64(4)})

	out := renderDefaults(t, s)
	wantLines(t, out, pgDefaultLines...)
	wantLines(t, out, "effective_io_concurrency = 1", "log_connections = off", "reserved_connections = 0")
	dontWantLines(t, out, "io_method", "transaction_timeout", "autovacuum_worker_slots")
}

func TestPostgresqlConf17(t *testing.T) {
	s := PostgresqlConf17()
	runPG(t, s, map[string]any{"reserved_connections": int64(4), "transaction_timeout": int64(300000)})

	out := renderDefaults(t, s)
	wantLines(t, out, pgDefaultLines...)
	wantLines(t, out, "effective_io_concurrency = 1", "log_connections = off",
		"reserved_connections = 0", "transaction_timeout = 0ms")
	dontWantLines(t, out, "io_method", "autovacuum_worker_slots")
}

func TestPostgresqlConf18(t *testing.T) {
	s := PostgresqlConf18()
	runPG(t, s, map[string]any{
		"reserved_connections":            int64(4),
		"transaction_timeout":             int64(300000),
		"io_method":                       "io_uring",
		"io_workers":                      int64(8),
		"autovacuum_vacuum_max_threshold": int64(50000000),
		"autovacuum_worker_slots":         int64(32),
		"log_connections":                 "receipt,authentication",
	})

	out := renderDefaults(t, s)
	wantLines(t, out, pgDefaultLines...)
	wantLines(t, out,
		"effective_io_concurrency = 16",
		"io_method = worker",
		"io_workers = 3",
		"autovacuum_worker_slots = 16",
		"autovacuum_vacuum_max_threshold = 100000000",
		"transaction_timeout = 0ms",
		"reserved_connections = 0",
		"log_connections = ''",
	)
}

// TestPostgresqlConfExtras checks the parts that only appear once the server
// has filled the cluster/extension fields.
func TestPostgresqlConfExtras(t *testing.T) {
	s := PostgresqlConf17()

	vals, _, err := s.Resolve(map[string]any{
		"synchronous_standby_names": "ANY 1 (pg-1, pg-2)",
		"extensions":                []any{"pg_stat_statements", "auto_explain"},
		"custom":                    map[string]any{"bgwriter_delay": "10ms"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := s.Render("conf", vals)
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, out,
		"synchronous_standby_names = 'ANY 1 (pg-1, pg-2)'",
		"shared_preload_libraries = 'pg_stat_statements,auto_explain'",
		"pg_stat_statements.max = 5000",
		"pg_stat_statements.track = top",
		"bgwriter_delay = 10ms",
		"# --- custom ---",
	)
}
