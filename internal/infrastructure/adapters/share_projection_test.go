package adapters

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// Anything an author can type free-form may hold a credential. The public
// projection is an allowlist, and these strings prove it: none of them may
// survive into a snapshot handed to a stranger with a link.
const (
	secretEnv       = "SECRET-ENV-PGPASSWORD"
	secretSQL       = "SECRET-PRIVATE-SCHEMA"
	secretArg       = "SECRET-CLI-TOKEN"
	secretOption    = "SECRET-CONF-VALUE"
	secretFile      = "SECRET-FILE-CONTENT"
	secretOptionKey = "SECRET-CONF-KEY"
)

func specWithSecrets() *domain.TestRun {
	return &domain.TestRun{
		Database: &domain.Database{
			Source: &domain.Database_Params{Params: &domain.DatabaseParams{
				Version: "17",
				Engine: &domain.DatabaseParams_Postgres{Postgres: &domain.PostgresParams{
					Replicas:     2,
					SyncReplicas: 1,
					Haproxy:      1,
					Pgbouncer:    true,
					// Free-form: whatever the author typed.
					MasterOptions:  map[string]string{secretOptionKey: secretOption},
					PatroniOptions: map[string]string{"password": secretOption},
				}},
			}},
		},
		Workload: &domain.Workload{
			StroppyVersion: "5.5.2",
			Segments: []*domain.Workload_Segment{{
				Name:   "tx",
				Script: "tpcc/tx",
				Sql:    secretSQL,
				Files:  []*domain.Workload_WorkloadFile{{Name: "seed.sql", Content: secretFile}},
				Execution: &domain.Workload_Execution{
					Vus:       proto.Uint32(8),
					Limit:     &domain.Workload_Execution_Duration{Duration: "5m"},
					ExtraArgs: []string{"--token=" + secretArg},
					Quiet:     proto.Bool(true),
				},
				Parameters: &domain.Workload_Parameters{
					PoolSize:            32,
					ScaleFactor:         10,
					DefaultInsertMethod: "plain_bulk",
					BulkSize:            1000,
					Steps:               []string{"load"},
					Env:                 map[string]string{"PGPASSWORD": secretEnv},
				},
			}},
		},
	}
}

func projectRun(t *testing.T) *models.SharedTestRun {
	t.Helper()
	b := NewRunSnapshotBuilder(nil, nil)
	rec := &models.TestRunRecord{
		Entity: &common.Entity{Id: "run-1", Name: "shared"},
		Spec:   specWithSecrets(),
	}
	view := b.sharedTestRun(context.Background(), rec)
	if view == nil {
		t.Fatal("sharedTestRun returned nil")
	}
	return view
}

// The whole point of the projection: free-form, author-supplied data never
// reaches a public share.
func TestSharedTestRunLeaksNoFreeFormFields(t *testing.T) {
	view := projectRun(t)
	raw, err := protojson.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rendered := string(raw)

	for name, secret := range map[string]string{
		"parameters.env":          secretEnv,
		"segment.sql":             secretSQL,
		"execution.extra_args":    secretArg,
		"engine *_options value":  secretOption,
		"engine *_options key":    secretOptionKey,
		"workload file content":   secretFile,
		"free-form option marker": "password",
	} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("%s leaked into the public snapshot: %s", name, rendered)
		}
	}
}

func TestSharedTestRunProjectsWorkloadKnobs(t *testing.T) {
	view := projectRun(t)
	if len(view.GetWorkloadSegments()) != 1 {
		t.Fatalf("segments = %d, want 1", len(view.GetWorkloadSegments()))
	}
	seg := view.GetWorkloadSegments()[0]
	if seg.GetScript() != "tpcc/tx" || seg.GetVus() != 8 || seg.GetDuration() != "5m" {
		t.Fatalf("execution not projected: %+v", seg)
	}
	if seg.GetPoolSize() != 32 || seg.GetScaleFactor() != 10 {
		t.Fatalf("stroppy parameters not projected: %+v", seg)
	}
	if seg.GetInsertMethod() != "plain_bulk" || seg.GetBulkSize() != 1000 {
		t.Fatalf("insert knobs not projected: %+v", seg)
	}
	if len(seg.GetSteps()) != 1 || seg.GetSteps()[0] != "load" || !seg.GetQuiet() {
		t.Fatalf("steps/flags not projected: %+v", seg)
	}
}

func TestSharedDatabaseProjectsTypedSizingOnly(t *testing.T) {
	db := projectRun(t).GetDatabase()
	if db == nil {
		t.Fatal("database not projected")
	}
	if db.GetVersion() != "17" {
		t.Fatalf("version = %q", db.GetVersion())
	}
	got := map[string]string{}
	for _, s := range db.GetSettings() {
		got[s.GetKey()] = s.GetValue()
	}
	want := map[string]string{
		"replicas":      "2",
		"sync_replicas": "1",
		"haproxy":       "1",
		"pgbouncer":     "true",
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("setting %q = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	// Zero-valued knobs stay out rather than rendering as noisy "0"/"false".
	if _, ok := got["patroni"]; ok {
		t.Fatalf("unset bool projected: %v", got)
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected settings: %v", got)
	}
}
