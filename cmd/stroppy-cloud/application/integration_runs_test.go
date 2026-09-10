//go:build integration

package application

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// runFixture builds a ready test (postgres single + tpcc + verified
// yandex profile) and returns the tenant base path, the token and the
// test id.
func runFixture(t *testing.T, e *e2e) (base, tok, testID string, tn tenant.Tenant) {
	t.Helper()
	owner := e.person(slug("owner")+"@example.com", "Owner")
	tn = e.tenant(owner, slug("runs"))
	tok = e.token(owner, tn)
	base = "/api/v1/t/" + tn.Slug

	var prof struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	e.want(e.req(http.MethodPost, base+"/providers", map[string]any{
		"name": "yc", "kind": "yandex",
		"settings":    map[string]any{"cloud_id": "b1gcloud000000000000", "folder_id": "b1gfolder00000000000", "zone": "ru-central1-a", "network": map[string]any{"kind": "create"}},
		"credentials": map[string]any{"sa_key_json": `{"id":"ajekey00000000000000","service_account_id":"ajesa000000000000000","created_at":"2026-01-01T00:00:00Z","key_algorithm":"RSA_2048","public_key":"-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----\n","private_key":"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----\n"}`},
	}, tok), http.StatusCreated, &prof)
	eventually(t, 10*time.Second, func() bool {
		e.want(e.req(http.MethodGet, base+"/providers/"+prof.ID, nil, tok), http.StatusOK, &prof)
		return prof.Status == "ready"
	})

	var db, wl, test struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	e.want(e.req(http.MethodPost, base+"/databases", map[string]any{"name": "pg-single", "kind": "postgres", "version": "17", "params": map[string]any{"version": "17"}}, tok), http.StatusCreated, &db)
	e.want(e.req(http.MethodPost, base+"/workloads", map[string]any{
		"name": "tpcc", "stroppy_version": "6.0.0", "protocol": "pg",
		"segments": []any{map[string]any{"name": "main", "workload": map[string]any{"script": "tpcc/tx", "scale_factor": 1}, "run": map[string]any{"vus": 8, "duration": "1m"}}},
	}, tok), http.StatusCreated, &wl)
	e.want(e.req(http.MethodPost, base+"/tests", map[string]any{
		"name": "pg tpcc", "database": map[string]any{"ref": map[string]any{"id": db.ID}}, "workload": map[string]any{"ref": map[string]any{"id": wl.ID}},
		"provider_profile_id": prof.ID, "sizes": map[string]any{"db": map[string]any{"size": "S"}, "runner": map[string]any{"size": "S"}}, "keep": "1h",
	}, tok), http.StatusCreated, &test)
	if test.Status != "ready" {
		t.Fatalf("test %+v", test)
	}
	return base, tok, test.ID, tn
}

type runView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Phase        string `json:"phase"`
	StatusReason string `json:"status_reason"`
	StandKept    bool   `json:"stand_kept"`
	Trigger      string `json:"trigger"`
	TriggerRef   struct {
		ParentRunID string `json:"parent_run_id"`
	} `json:"trigger_ref"`
	TestRef struct {
		ID string `json:"id"`
	} `json:"test_ref"`
	Snapshot struct {
		Machines []struct {
			Name string `json:"name"`
			Role string `json:"role"`
			CPU  int    `json:"cpu"`
		} `json:"machines"`
	} `json:"snapshot"`
	RunSpec struct {
		Values map[string]any `json:"values"`
	} `json:"run_spec"`
	Summary struct {
		DBKind        string             `json:"db_kind"`
		TopologyLabel string             `json:"topology_label"`
		NodeCount     int                `json:"node_count"`
		ProgressPct   float64            `json:"progress_pct"`
		Headline      map[string]float64 `json:"headline"`
	} `json:"summary"`
	Result struct {
		Segments []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"segments"`
	} `json:"result"`
	IsFavorite bool   `json:"is_favorite"`
	Duration   string `json:"duration"`
}

func TestE2ERuns(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, tn := runFixture(t, e)
	ctx := e.ctx

	var launched runView
	t.Run("launch compiles the RunSpec and starts the pipeline", func(t *testing.T) {
		r := e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{"name": "first", "labels": map[string]any{"ci": "1"}}, tok)
		e.want(r, http.StatusCreated, &launched)
		if launched.Status != "pending" || launched.Phase != "queued" || launched.Trigger != "api" || launched.TestRef.ID != testID {
			t.Fatalf("launched %+v", launched)
		}
		if len(launched.Snapshot.Machines) != 2 || launched.Summary.NodeCount != 2 || launched.Summary.DBKind != "postgres" {
			t.Fatalf("snapshot %+v", launched.Snapshot)
		}
		fr, ok := e.graphene.runOf(launched.ID)
		if !ok || fr.pipeline != "stroppy-run" || fr.namespace != tn.GrapheneNamespace {
			t.Fatalf("graphene run %+v", fr)
		}
		if !strings.Contains(string(fr.params), `"credentials_secret":"provider-`) || !strings.Contains(string(fr.params), "@${ip:role:db}:5432") {
			t.Fatalf("params %s", fr.params)
		}
		if launched.RunSpec.Values["run_id"] != launched.ID {
			t.Fatalf("run_spec %v", launched.RunSpec.Values["run_id"])
		}
	})

	t.Run("idempotency key replays the same run", func(t *testing.T) {
		var a, b runView
		req := func() resp {
			r, _ := http.NewRequestWithContext(ctx, http.MethodPost, e.ts.URL+base+"/tests/"+testID+":launch", strings.NewReader(`{"name":"idem"}`))
			r.Header.Set("Authorization", "Bearer "+tok)
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", "key-"+testID)
			res, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			raw, _ := io.ReadAll(res.Body)
			return resp{Status: res.StatusCode, Body: raw}
		}
		e.want(req(), http.StatusCreated, &a)
		e.want(req(), http.StatusCreated, &b)
		if a.ID != b.ID {
			t.Fatalf("idempotency: %s != %s", a.ID, b.ID)
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+a.ID+":cancel", nil, tok), http.StatusOK, nil)
	})

	t.Run("projection follows the event stream to completion", func(t *testing.T) {
		id := launched.ID
		if n := e.app.services.Projector.Tick(ctx); n < 1 {
			t.Fatalf("projector started %d followers", n)
		}
		e.graphene.emit(id, "run-started", "", nil)
		e.graphene.milestone(id, "phase.started", map[string]any{"phase": "provisioning"})
		e.graphene.milestone(id, "machine.ready", map[string]any{"machine": "db-1", "role": "db", "private_ip": "10.130.0.5"})
		e.graphene.milestone(id, "machine.ready", map[string]any{"machine": "runner-1", "role": "runner", "private_ip": "10.130.0.6"})
		e.graphene.milestone(id, "phase.finished", map[string]any{"phase": "provisioning"})
		e.graphene.milestone(id, "phase.started", map[string]any{"phase": "deploying"})
		e.graphene.milestone(id, "container.ready", map[string]any{"container": "db-1-postgres", "role": "db", "machine": "db-1"})
		e.graphene.milestone(id, "phase.finished", map[string]any{"phase": "deploying"})
		e.graphene.milestone(id, "phase.started", map[string]any{"phase": "workload"})
		e.graphene.milestone(id, "segment.started", map[string]any{"segment": "main", "script": "tpcc/tx"})

		var mid runView
		eventually(t, 10*time.Second, func() bool {
			e.want(e.req(http.MethodGet, base+"/runs/"+id, nil, tok), http.StatusOK, &mid)
			return mid.Status == "running" && mid.Phase == "workload"
		})
		var ov struct {
			Source   string `json:"source"`
			Machines []struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Address string `json:"address"`
			} `json:"machines"`
			Components []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"components"`
			WorkloadSegments []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"workload_segments"`
			ProgressPct float64 `json:"progress_pct"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+id+"/overview", nil, tok), http.StatusOK, &ov)
		ready := map[string]string{}
		for _, m := range ov.Machines {
			ready[m.Name] = m.Status + "@" + m.Address
		}
		if ov.Source != "persisted" || ready["db-1"] != "ready@10.130.0.5" || len(ov.WorkloadSegments) != 1 || ov.WorkloadSegments[0].Status != "running" || ov.ProgressPct <= 0 {
			t.Fatalf("overview %+v", ov)
		}
		comp := map[string]string{}
		for _, c := range ov.Components {
			comp[c.ID] = c.Status
		}
		if comp["db-1-postgres"] != "ready" || comp["db-1-node-exporter"] != "pending" {
			t.Fatalf("components %+v", comp)
		}

		e.graphene.milestone(id, "segment.finished", map[string]any{"segment": "main", "metrics": map[string]any{"tps": 1234.5}})
		e.graphene.milestone(id, "phase.finished", map[string]any{"phase": "workload"})
		e.graphene.milestone(id, "stand.kept", map[string]any{"keep": "1h"})
		e.graphene.finish(id, "run-completed", "completed", spec.Result{
			Metrics:  map[string]spec.MetricValue{"tps": {Value: 1234.5}},
			Segments: []spec.SegmentResult{{Name: "main", Status: spec.SegmentCompleted, Metrics: map[string]spec.MetricValue{"tps": {Value: 1234.5}}}},
			Summary:  spec.Summary{TPS: 1234.5, LatencyP95Ms: 12},
		})
		var done runView
		eventually(t, 10*time.Second, func() bool {
			e.want(e.req(http.MethodGet, base+"/runs/"+id, nil, tok), http.StatusOK, &done)
			return done.Status == "completed"
		})
		if done.Phase != "done" || done.Summary.Headline["tps"] != 1234.5 || done.Summary.ProgressPct != 100 || !done.StandKept || done.Duration == "" {
			t.Fatalf("done %+v", done)
		}
		if len(done.Result.Segments) != 1 || done.Result.Segments[0].Status != "completed" {
			t.Fatalf("result %+v", done.Result)
		}
		var events struct {
			Data []struct {
				Kind  string `json:"kind"`
				Title string `json:"title"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+id+"/events", nil, tok), http.StatusOK, &events)
		kinds := map[string]bool{}
		for _, ev := range events.Data {
			kinds[ev.Kind] = true
		}
		if !kinds["run-started"] || !kinds["phase.started"] || !kinds["segment.finished"] || !kinds["run-completed"] {
			t.Fatalf("events %+v", events)
		}
		var tree struct {
			Ref      string `json:"ref"`
			Children []struct {
				Ref string `json:"ref"`
			} `json:"children"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+id+"/tree", nil, tok), http.StatusOK, &tree)
		if tree.Ref != "run/"+id || len(tree.Children) != 1 {
			t.Fatalf("tree %+v", tree)
		}
		var arts struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Kind string `json:"kind"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+id+"/artifacts", nil, tok), http.StatusOK, &arts)
		if len(arts.Data) != 1 || arts.Data[0].Kind != "stroppy_raw" {
			t.Fatalf("artifacts %+v", arts)
		}
		if r := e.req(http.MethodGet, base+"/runs/"+id+"/artifacts/"+arts.Data[0].ID, nil, tok); r.Status != http.StatusOK || string(r.Body) != "{}" {
			t.Fatalf("download %d %s", r.Status, r.Body)
		}
	})

	t.Run("keep extend and release go to the stand", func(t *testing.T) {
		id := launched.ID
		var got runView
		e.want(e.req(http.MethodPost, base+"/runs/"+id+":keep-extend", map[string]any{"duration": "2h"}, tok), http.StatusOK, &got)
		e.problem(e.req(http.MethodPost, base+"/runs/"+id+":keep-extend", map[string]any{"duration": "999h"}, tok), http.StatusUnprocessableEntity, "limit_exceeded")
		e.want(e.req(http.MethodPost, base+"/runs/"+id+":keep-release", nil, tok), http.StatusOK, &got)
		if got.StandKept {
			t.Fatalf("still kept %+v", got)
		}
		e.problem(e.req(http.MethodPost, base+"/runs/"+id+":keep-release", nil, tok), http.StatusConflict, "conflict")
		e.graphene.mu.Lock()
		cmds := strings.Join(e.graphene.commands, ",")
		e.graphene.mu.Unlock()
		if !strings.Contains(cmds, "stand/stroppy-run extend") || !strings.Contains(cmds, "stand/stroppy-run release") {
			t.Fatalf("commands %s", cmds)
		}
	})

	t.Run("list, facets, favorites, patch, export", func(t *testing.T) {
		id := launched.ID
		e.want(e.req(http.MethodPut, base+"/favorites/run/"+id, nil, tok), http.StatusNoContent, nil)
		var list struct {
			Data []runView `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs?status=completed&favorites=true", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 || list.Data[0].ID != id || !list.Data[0].IsFavorite {
			t.Fatalf("list %+v", list)
		}
		e.want(e.req(http.MethodGet, base+"/runs?sort=tps&order=desc&kind=postgres&labels=ci:1", nil, tok), http.StatusOK, &list)
		if len(list.Data) != 1 {
			t.Fatalf("filtered list %+v", list)
		}
		var facets struct {
			Data []struct {
				Field  string `json:"field"`
				Values []struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				} `json:"values"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs:facets", nil, tok), http.StatusOK, &facets)
		if len(facets.Data) == 0 {
			t.Fatalf("facets %+v", facets)
		}
		var patched runView
		e.want(e.req(http.MethodPatch, base+"/runs/"+id, map[string]any{"name": "renamed", "notes": "# hi", "rating": map[string]any{"global": true}}, tok), http.StatusOK, &patched)
		if patched.Name != "renamed" {
			t.Fatalf("patched %+v", patched)
		}
		if r := e.req(http.MethodGet, base+"/runs/"+id+"/export?format=md", nil, tok); r.Status != http.StatusOK || !strings.Contains(string(r.Body), "# renamed") {
			t.Fatalf("export %d %s", r.Status, r.Body)
		}
		var history struct {
			Data  []runView `json:"data"`
			Trend struct {
				Points []struct {
					Value float64 `json:"value"`
				} `json:"points"`
			} `json:"trend"`
		}
		e.want(e.req(http.MethodGet, base+"/tests/"+testID+"/runs", nil, tok), http.StatusOK, &history)
		if len(history.Data) < 2 || len(history.Trend.Points) != 1 || history.Trend.Points[0].Value != 1234.5 {
			t.Fatalf("history %+v", history)
		}
	})

	t.Run("cancel, delete, rerun, save-as-test", func(t *testing.T) {
		var second runView
		e.want(e.req(http.MethodPost, base+"/runs/"+launched.ID+":rerun", map[string]any{}, tok), http.StatusCreated, &second)
		if second.TriggerRef.ParentRunID != launched.ID || second.Status != "pending" {
			t.Fatalf("rerun %+v", second)
		}
		e.problem(e.req(http.MethodDelete, base+"/runs/"+second.ID, nil, tok), http.StatusConflict, "conflict")
		e.app.services.Projector.Tick(ctx)
		e.graphene.emit(second.ID, "run-started", "", nil)
		var cancelled runView
		e.want(e.req(http.MethodPost, base+"/runs/"+second.ID+":cancel", nil, tok), http.StatusOK, &cancelled)
		eventually(t, 10*time.Second, func() bool {
			e.want(e.req(http.MethodGet, base+"/runs/"+second.ID, nil, tok), http.StatusOK, &cancelled)
			return cancelled.Status == "cancelled"
		})
		e.problem(e.req(http.MethodPost, base+"/runs/"+second.ID+":cancel", nil, tok), http.StatusConflict, "conflict")
		e.want(e.req(http.MethodDelete, base+"/runs/"+second.ID, nil, tok), http.StatusNoContent, nil)
		e.problem(e.req(http.MethodGet, base+"/runs/"+second.ID, nil, tok), http.StatusNotFound, "not_found")
		e.graphene.mu.Lock()
		deleted := strings.Join(e.graphene.deleted, ",")
		e.graphene.mu.Unlock()
		if !strings.Contains(deleted, "run/"+second.ID) {
			t.Fatalf("graphene delete %s", deleted)
		}

		var resumed struct {
			ID      string `json:"id"`
			Resumed bool   `json:"resumed"`
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+launched.ID+":rerun-resume", map[string]any{}, tok), http.StatusCreated, &resumed)
		if resumed.Resumed {
			t.Fatalf("resume should degrade to rerun: %+v", resumed)
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+resumed.ID+":cancel", nil, tok), http.StatusOK, nil)

		var saved struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Name   string `json:"name"`
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+launched.ID+":save-as-test", map[string]any{"name": "from run", "save_database_as": "pg-from-run"}, tok), http.StatusCreated, &saved)
		if saved.Status != "ready" || saved.Name != "from run" {
			t.Fatalf("saved %+v", saved)
		}
	})

	t.Run("limits: concurrent runs and viewer access", func(t *testing.T) {
		// Cancelled runs of the earlier steps settle through the projector.
		var live struct {
			Data []runView `json:"data"`
		}
		eventually(t, 10*time.Second, func() bool {
			e.app.services.Projector.Tick(ctx)
			e.want(e.req(http.MethodGet, base+"/runs?status=pending,running,cancelling", nil, tok), http.StatusOK, &live)
			return len(live.Data) == 0
		})
		viewer := e.person(slug("viewer")+"@example.com", "Viewer")
		e.member(tn, viewer, tenant.RoleViewer)
		vtok := e.token(viewer, tn)
		e.problem(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, vtok), http.StatusForbidden, "forbidden")
		e.want(e.req(http.MethodGet, base+"/runs", nil, vtok), http.StatusOK, nil)

		var ids []string
		for i := 0; i < 3; i++ {
			var r runView
			e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusCreated, &r)
			ids = append(ids, r.ID)
		}
		e.problem(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{}, tok), http.StatusUnprocessableEntity, "limit_exceeded")
		for _, id := range ids {
			e.want(e.req(http.MethodPost, base+"/runs/"+id+":cancel", nil, tok), http.StatusOK, nil)
		}
		// Keep beyond the tenant ceiling is a fit issue of the resolved test.
		e.problem(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{"keep": "400h"}, tok), http.StatusUnprocessableEntity, "invalid")
	})
}
