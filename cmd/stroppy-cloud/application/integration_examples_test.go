//go:build integration

package application

import (
	"net/http"
	"testing"
)

func TestE2EExamples(t *testing.T) {
	e := e2eServer(t)
	base, tok, _, _ := runFixture(t, e)

	var gallery struct {
		Data []struct {
			ID       string         `json:"id"`
			Kind     string         `json:"kind"`
			Document map[string]any `json:"document"`
		} `json:"data"`
	}
	e.want(e.req(http.MethodGet, "/api/v1/catalog/examples", nil, tok), http.StatusOK, &gallery)
	if len(gallery.Data) < 6 || gallery.Data[0].Document["kind"] == nil {
		t.Fatalf("gallery %+v", gallery)
	}
	e.want(e.req(http.MethodGet, "/api/v1/catalog/examples?kind=suite", nil, tok), http.StatusOK, &gallery)
	if len(gallery.Data) != 1 || gallery.Data[0].ID != "pg-vs-cockroach" {
		t.Fatalf("suite examples %+v", gallery)
	}

	t.Run("clone every kind into the library", func(t *testing.T) {
		for _, id := range []string{"pg-single", "tpcc-smoke", "pg-self-check", "pg-vs-cockroach"} {
			var created struct {
				Created []struct {
					Kind string `json:"kind"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"created"`
			}
			e.want(e.req(http.MethodPost, base+"/examples/"+id+":clone", map[string]any{}, tok), http.StatusCreated, &created)
			if len(created.Created) != 1 || created.Created[0].ID == "" {
				t.Fatalf("%s: %+v", id, created)
			}
		}
		var suites struct {
			Data []struct {
				Name          string           `json:"name"`
				ComputedCells []map[string]any `json:"computed_cells"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/suites?search=CockroachDB", nil, tok), http.StatusOK, &suites)
		if len(suites.Data) != 1 || len(suites.Data[0].ComputedCells) != 4 {
			t.Fatalf("cloned suite %+v", suites)
		}
		var named struct {
			Created []struct {
				Name string `json:"name"`
			} `json:"created"`
		}
		e.want(e.req(http.MethodPost, base+"/examples/pg-single:clone", map[string]any{"name": "my pg"}, tok), http.StatusCreated, &named)
		if named.Created[0].Name != "my pg" {
			t.Fatalf("named clone %+v", named)
		}
		e.problem(e.req(http.MethodPost, base+"/examples/nope:clone", map[string]any{}, tok), http.StatusNotFound, "not_found")
	})

	t.Run("quick-run needs a provider and launches without cloning", func(t *testing.T) {
		e.problem(e.req(http.MethodPost, base+"/examples/pg-self-check:quick-run", map[string]any{}, tok), http.StatusUnprocessableEntity, "invalid")
		var profiles struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		e.want(e.req(http.MethodGet, base+"/providers", nil, tok), http.StatusOK, &profiles)
		var r runView
		e.want(e.req(http.MethodPost, base+"/examples/pg-self-check:quick-run", map[string]any{"provider_profile_id": profiles.Data[0].ID}, tok), http.StatusCreated, &r)
		if r.Status != "pending" || r.TestRef.ID != "00000000-0000-0000-0000-000000000000" || r.Summary.DBKind != "postgres" {
			t.Fatalf("quick run %+v", r)
		}
		e.want(e.req(http.MethodPost, base+"/runs/"+r.ID+":cancel", nil, tok), http.StatusOK, nil)
		e.problem(e.req(http.MethodPost, base+"/examples/tpcc-smoke:quick-run", map[string]any{"provider_profile_id": profiles.Data[0].ID}, tok), http.StatusUnprocessableEntity, "invalid")
	})
}
