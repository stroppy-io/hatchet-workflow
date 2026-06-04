package victoria

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLogsClientWritePostsJSONLines(t *testing.T) {
	var (
		gotPath        string
		gotAuth        string
		gotAccountID   string
		gotContentType string
		gotRows        []map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccountID = r.Header.Get("AccountID")
		gotContentType = r.Header.Get("Content-Type")

		dec := json.NewDecoder(r.Body)
		for {
			var row map[string]any
			if err := dec.Decode(&row); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Fatalf("decode request row: %v", err)
			}
			gotRows = append(gotRows, row)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewLogsClient(server.URL, "secret")
	err := client.Write(context.Background(), []*monitor.LogLine{
		nil,
		{RunId: "run-1"},
		{
			ObservedAt:            timestamppb.New(time.Date(2026, 6, 3, 10, 0, 0, 123, time.UTC)),
			RunId:                 "run-1",
			LineNo:                7,
			NodeExecutionId:       "deploy-step-1",
			ParentNodeExecutionId: "component/postgres-master",
			Phase:                 "execute_deployment_plan",
			StageName:             "call_cmd: install postgres",
			ComponentId:           "postgres-master",
			MachineId:             "node-1",
			StepId:                "120_install",
			Action:                "call_cmd",
			Mentions:              []string{"postgres", "pgdg"},
			Source:                monitor.Source_SOURCE_COMMAND,
			Unit:                  "call_cmd",
			Stream:                monitor.Stream_STREAM_STDERR,
			Line:                  "started",
		},
	})
	if err != nil {
		t.Fatalf("write logs: %v", err)
	}

	if got, want := gotPath, "/insert/jsonline"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if got, want := gotAuth, "Bearer secret"; got != want {
		t.Fatalf("authorization = %q, want %q", got, want)
	}
	if got, want := gotAccountID, "0"; got != want {
		t.Fatalf("AccountID = %q, want %q", got, want)
	}
	if !strings.HasPrefix(gotContentType, "application/x-ndjson") {
		t.Fatalf("content-type = %q, want application/x-ndjson", gotContentType)
	}
	if got, want := len(gotRows), 1; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}

	row := gotRows[0]
	for key, want := range map[string]any{
		"_msg":                     "started",
		"run_id":                   "run-1",
		"line_no":                  float64(7),
		"node_execution_id":        "deploy-step-1",
		"parent_node_execution_id": "component/postgres-master",
		"phase":                    "execute_deployment_plan",
		"stage_name":               "call_cmd: install postgres",
		"component_id":             "postgres-master",
		"machine_id":               "node-1",
		"step_id":                  "120_install",
		"action":                   "call_cmd",
		"mentions":                 "postgres,pgdg",
		"source":                   "command",
		"unit":                     "call_cmd",
		"stream":                   "stderr",
	} {
		if got := row[key]; got != want {
			t.Fatalf("row[%s] = %#v, want %#v", key, got, want)
		}
	}
	if got, want := row["_time"], "2026-06-03T10:00:00.000000123Z"; got != want {
		t.Fatalf("row[_time] = %#v, want %#v", got, want)
	}
}
