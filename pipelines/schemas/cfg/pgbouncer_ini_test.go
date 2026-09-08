package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestPgbouncerIni1(t *testing.T) {
	full := map[string]any{
		"db_name":                   "stroppy",
		"db_host":                   "10.0.0.11",
		"db_port":                   int64(5432),
		"db_dbname":                 "stroppy",
		"listen_addr":               "*",
		"listen_port":               int64(6432),
		"auth_type":                 "scram-sha-256",
		"auth_file":                 "/etc/pgbouncer/userlist.txt",
		"auth_user":                 "pgbouncer",
		"auth_query":                "SELECT usename, passwd FROM pg_shadow WHERE usename=$1",
		"pool_mode":                 "transaction",
		"max_client_conn":           int64(5000),
		"default_pool_size":         int64(200),
		"min_pool_size":             int64(20),
		"reserve_pool_size":         int64(50),
		"reserve_pool_timeout":      3.0,
		"max_db_connections":        int64(400),
		"max_user_connections":      int64(400),
		"server_idle_timeout":       120.0,
		"server_lifetime":           1800.0,
		"server_reset_query":        "",
		"query_wait_timeout":        30.0,
		"client_idle_timeout":       0.0,
		"ignore_startup_parameters": "extra_float_digits,options",
		"log_connections":           "0",
		"log_disconnections":        "0",
		"stats_period":              int64(15),
		"admin_users":               "postgres",
		"stats_users":               "postgres,exporter",
	}

	schematest.Run(t, PgbouncerIni1(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"pool_mode": "pooled"}, Code: "CHOICE_NOT_ALLOWED", Path: "pool_mode"},
			{Value: map[string]any{"listen_port": int64(0)}, Code: "GTE_VIOLATED", Path: "listen_port"},
			{Value: map[string]any{"auth_file": "userlist.txt"}, Code: "PATTERN_MISMATCH", Path: "auth_file"},
			{
				Value: map[string]any{"server_reset_query": "DISCARD ALL"},
				Code:  "RULE_VIOLATED", Path: "reset-query-vs-pool-mode",
			},
			{
				Value: map[string]any{"auth_query": "SELECT 1"},
				Code:  "RULE_VIOLATED", Path: "auth-query-needs-user",
			},
			{
				Value: map[string]any{"min_pool_size": int64(100)},
				Code:  "RULE_VIOLATED", Path: "min-vs-default-pool",
			},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"[databases]",
			"[pgbouncer]",
			"listen_addr = *",
			"listen_port = ",
			"auth_type = scram-sha-256",
			"pool_mode = transaction",
			"max_client_conn = ",
			"default_pool_size = ",
			"server_reset_query = ",
			"stats_period = ",
		},
	})

	out := renderDefaults(t, PgbouncerIni1())
	wantLines(t, out,
		"listen_port = 6432",
		"auth_file = /etc/pgbouncer/userlist.txt",
		"max_client_conn = 100",
		"default_pool_size = 20",
		"min_pool_size = 0",
		"reserve_pool_size = 0",
		"reserve_pool_timeout = 5",
		"max_db_connections = 0",
		"max_user_connections = 0",
		"server_idle_timeout = 600",
		"server_lifetime = 3600",
		"query_wait_timeout = 120",
		"client_idle_timeout = 0",
		"log_connections = 1",
		"log_disconnections = 1",
		"stats_period = 60",
	)
	dontWantLines(t, out, "auth_user =", "auth_query =")

	vals, _, err := PgbouncerIni1().Resolve(full)
	if err != nil {
		t.Fatal(err)
	}

	got, err := PgbouncerIni1().Render("conf", vals)
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, got,
		"stroppy = host=10.0.0.11 port=5432 dbname=stroppy",
		"auth_user = pgbouncer",
		"auth_query = SELECT usename, passwd FROM pg_shadow WHERE usename=$1",
		"log_connections = 0",
	)
}
