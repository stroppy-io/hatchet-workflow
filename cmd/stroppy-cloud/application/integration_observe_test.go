//go:build integration

package application

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/transport/ws"
)

func TestE2EObserve(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixture(t, e)
	var r runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", map[string]any{"name": "observed"}, tok), http.StatusCreated, &r)
	finishRun(t, e, r.ID, 500)
	eventually(t, 10*time.Second, func() bool {
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID, nil, tok), http.StatusOK, &r)
		return r.Status == "completed"
	})
	now := time.Now().UTC()
	e.victoria.mu.Lock()
	e.victoria.lines = []map[string]string{
		{"_time": now.Add(-2 * time.Second).Format(time.RFC3339Nano), "_msg": "ready to accept connections", "role": "db", "machine": "db-1", "container": "db-1-postgres", "stream": "stdout", "level": "info", "pid": "42"},
		{"_time": now.Add(-1 * time.Second).Format(time.RFC3339Nano), "_msg": "segment main started", "role": "runner", "machine": "runner-1", "stream": "pipeline", "phase": "workload", "segment": "main"},
	}
	e.victoria.mu.Unlock()

	t.Run("typed logs carry the run scope and the filters", func(t *testing.T) {
		var page struct {
			Data []struct {
				Message string            `json:"message"`
				Role    string            `json:"role"`
				Fields  map[string]string `json:"fields"`
			} `json:"data"`
			Older *string `json:"older"`
			Newer *string `json:"newer"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?role=db,runner&stream=stdout&q=ready&limit=50", nil, tok), http.StatusOK, &page)
		if len(page.Data) != 2 || page.Data[0].Message != "segment main started" || page.Data[1].Fields["pid"] != "42" || page.Older == nil || page.Newer == nil {
			t.Fatalf("logs %+v", page)
		}
		q := e.victoria.last()
		for _, want := range []string{`stroppy_run_id:="` + r.ID + `"`, `role:in("db","runner")`, `stream:in("stdout")`, `_msg:"ready"`} {
			if !strings.Contains(q, want) {
				t.Fatalf("query %q lacks %q", q, want)
			}
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs?cursor="+*page.Older+"&direction=older", nil, tok), http.StatusOK, &page)
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/logs:raw", map[string]any{"query": `_msg:"ready" AND role:db`}, tok), http.StatusOK, &page)
		if q := e.victoria.last(); !strings.HasPrefix(q, "/select/logsql/query stroppy_run_id:=") || !strings.Contains(q, `AND (_msg:"ready" AND role:db)`) {
			t.Fatalf("raw query %q", q)
		}
		e.problem(e.req(http.MethodPost, base+"/runs/"+r.ID+"/logs:raw", map[string]any{"query": ""}, tok), http.StatusUnprocessableEntity, "invalid")
		var facets struct {
			Data []struct {
				Field  string `json:"field"`
				Values []struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				} `json:"values"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/logs:facets", nil, tok), http.StatusOK, &facets)
		if len(facets.Data) != 1 || facets.Data[0].Field != "role" || facets.Data[0].Values[0].Count != 3 {
			t.Fatalf("facets %+v", facets)
		}
	})

	t.Run("metrics: catalog keys, scope injection, raw PromQL, per-key errors", func(t *testing.T) {
		var m struct {
			Window struct {
				Start   time.Time `json:"start"`
				Segment string    `json:"segment"`
			} `json:"window"`
			Series []struct {
				Key        string      `json:"key"`
				Machine    string      `json:"machine"`
				Points     [][]float64 `json:"points"`
				Aggregates struct {
					Avg float64 `json:"avg"`
					Max float64 `json:"max"`
					P95 float64 `json:"p95"`
				} `json:"aggregates"`
			} `json:"series"`
			Errors []map[string]string `json:"errors"`
		}
		e.want(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics?keys=tps,node_cpu_usage&segment=main", nil, tok), http.StatusOK, &m)
		if len(m.Series) != 2 || m.Series[0].Machine != "db-1" || len(m.Series[0].Points) != 3 || m.Series[0].Aggregates.Avg != 20 || m.Series[0].Aggregates.Max != 30 || m.Window.Segment != "main" {
			t.Fatalf("metrics %+v", m)
		}
		if q := e.victoria.last(); !strings.Contains(q, `stroppy_run_id="`+r.ID+`"`) {
			t.Fatalf("scope missing in %q", q)
		}
		e.problem(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics?keys=nope", nil, tok), http.StatusUnprocessableEntity, "invalid")
		e.problem(e.req(http.MethodGet, base+"/runs/"+r.ID+"/metrics?segment=missing", nil, tok), http.StatusNotFound, "not_found")

		var raw map[string]any
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/metrics:raw", map[string]any{"query": `rate(pg_stat_database_xact_commit{datname="postgres"}[1m]) / sum by (machine) (up)`, "start": now.Add(-time.Hour), "end": now}, tok), http.StatusOK, &raw)
		if raw["status"] != "success" {
			t.Fatalf("raw %+v", raw)
		}
		q := e.victoria.last()
		if !strings.Contains(q, `pg_stat_database_xact_commit{stroppy_run_id="`+r.ID+`",datname="postgres"}`) || !strings.Contains(q, `up{stroppy_run_id="`+r.ID+`"}`) || !strings.Contains(q, "sum by (machine)") {
			t.Fatalf("scoped promql %q", q)
		}
		e.problem(e.req(http.MethodPost, base+"/runs/"+r.ID+"/metrics:raw", map[string]any{"query": "boom", "start": now.Add(-time.Hour), "end": now}, tok), http.StatusUnprocessableEntity, "invalid")
	})

	t.Run("grafana session links the dashboards", func(t *testing.T) {
		var gs struct {
			Dashboards []struct {
				ID         string `json:"id"`
				URL        string `json:"url"`
				PerMachine bool   `json:"per_machine"`
			} `json:"dashboards"`
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+"/grafana-session", nil, tok), http.StatusOK, &gs)
		if len(gs.Dashboards) != 2 || !strings.HasPrefix(gs.Dashboards[0].URL, "/grafana/d/stroppy-run?") || !strings.Contains(gs.Dashboards[0].URL, "var-run_id="+r.ID) || !gs.Dashboards[1].PerMachine {
			t.Fatalf("grafana %+v", gs)
		}
	})

	t.Run("ws log tail and metrics topics", func(t *testing.T) {
		c := dialWS(t, e, tok)
		c.send(ws.Frame{Type: "subscribe", SubID: "logs", Topic: "run.logs/" + r.ID})
		f := c.next(5*time.Second, "event", "logs")
		if !strings.Contains(string(f.Payload), "ready to accept") || f.Cursor == "" {
			t.Fatalf("log tail %+v", f)
		}
		c.send(ws.Frame{Type: "subscribe", SubID: "m", Topic: "run.metrics/" + r.ID})
		f = c.next(5*time.Second, "event", "m")
		if !strings.Contains(string(f.Payload), `"series"`) {
			t.Fatalf("metrics topic %+v", f)
		}
	})
}
