package cfg

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/internal/schematest"
)

func TestEtcd3(t *testing.T) {
	full := map[string]any{
		"name":                        "etcd-1",
		"data_dir":                    "/var/lib/etcd/stroppy",
		"listen_peer_urls":            "http://0.0.0.0:2380",
		"listen_client_urls":          "http://0.0.0.0:2379",
		"initial_advertise_peer_urls": "http://10.0.0.21:2380",
		"advertise_client_urls":       "http://10.0.0.21:2379",
		"initial_cluster": []any{
			"etcd-1=http://10.0.0.21:2380",
			"etcd-2=http://10.0.0.22:2380",
			"etcd-3=http://10.0.0.23:2380",
		},
		"initial_cluster_state":     "new",
		"initial_cluster_token":     "stroppy-etcd",
		"heartbeat_interval":        int64(200),
		"election_timeout":          int64(2000),
		"snapshot_count":            int64(50000),
		"quota_backend_bytes":       int64(8589934592),
		"auto_compaction_mode":      "revision",
		"auto_compaction_retention": "1000",
		"max_request_bytes":         int64(3145728),
		"enable_v2":                 "false",
		"custom":                    map[string]any{"log_level": "warn"},
	}

	schematest.Run(t, Etcd3(), schematest.Cases{
		Valid: []map[string]any{{}, full},
		Invalid: []schematest.Invalid{
			{Value: map[string]any{"heartbeat_interval": int64(0)}, Code: "GTE_VIOLATED", Path: "heartbeat_interval"},
			{Value: map[string]any{"initial_cluster_state": "rejoin"}, Code: "CHOICE_NOT_ALLOWED", Path: "initial_cluster_state"},
			{Value: map[string]any{"data_dir": "relative/path"}, Code: "PATTERN_MISMATCH", Path: "data_dir"},
			{
				Value: map[string]any{"heartbeat_interval": int64(1000), "election_timeout": int64(1000)},
				Code:  "RULE_VIOLATED", Path: "election-vs-heartbeat",
			},
			{Value: map[string]any{"custom": map[string]any{"Bad Key": "x"}}, Code: "RULE_VIOLATED", Path: "custom-keys"},
			{Value: map[string]any{"junk": 1}, Code: "UNKNOWN_FIELD", Path: "junk"},
		},
		Render: "conf",
		Contains: []string{
			"name: ",
			"data-dir: ",
			"listen-peer-urls: ",
			"listen-client-urls: ",
			"initial-cluster-state: new",
			"initial-cluster-token: ",
			"heartbeat-interval: ",
			"election-timeout: ",
			"snapshot-count: ",
			"quota-backend-bytes: ",
			"auto-compaction-mode: ",
			"max-request-bytes: ",
			"enable-v2: false",
		},
	})

	out := renderDefaults(t, Etcd3())
	wantLines(t, out,
		"name: default",
		"data-dir: /var/lib/etcd",
		"listen-peer-urls: http://0.0.0.0:2380",
		"listen-client-urls: http://0.0.0.0:2379",
		"initial-cluster-token: etcd-cluster",
		"heartbeat-interval: 100",
		"election-timeout: 1000",
		"snapshot-count: 100000",
		"quota-backend-bytes: 0",
		"auto-compaction-mode: periodic",
		`auto-compaction-retention: "1h"`,
		"max-request-bytes: 1572864",
	)
	dontWantLines(t, out, "initial-cluster:", "advertise-")

	vals, _, err := Etcd3().Resolve(full)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Etcd3().Render("conf", vals)
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, got,
		"initial-advertise-peer-urls: http://10.0.0.21:2380",
		"advertise-client-urls: http://10.0.0.21:2379",
		"initial-cluster: etcd-1=http://10.0.0.21:2380,etcd-2=http://10.0.0.22:2380,etcd-3=http://10.0.0.23:2380",
		"log_level: warn",
	)
}
