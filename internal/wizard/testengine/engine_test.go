package testengine

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestInitialForm(t *testing.T) {
	e := New()
	form, err := e.InitialForm(context.Background(), "tenant", "demo", nil)
	if err != nil {
		t.Fatalf("InitialForm error: %v", err)
	}
	if form == nil {
		t.Fatal("InitialForm returned nil Filled")
	}
	if form.GetSchema() == nil {
		t.Fatal("InitialForm Filled has no schema")
	}
	if form.GetValues() == nil {
		t.Fatal("InitialForm Filled has no values")
	}
}

func TestComputeInitialFormReady(t *testing.T) {
	e := New()
	form, err := e.InitialForm(context.Background(), "tenant", "demo", nil)
	if err != nil {
		t.Fatalf("InitialForm error: %v", err)
	}

	newForm, topo, errs, ready, err := e.Compute(context.Background(), "tenant", form)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("Compute returned %d field errors: %v", len(errs), errs)
	}
	if !ready {
		t.Fatal("Compute on initial form should be ready")
	}
	if newForm == nil {
		t.Fatal("Compute returned nil form")
	}
	if topo == nil {
		t.Fatal("Compute returned nil topology")
	}
	if len(topo.GetInstances()) < 1 {
		t.Fatalf("want >=1 instance, got %d", len(topo.GetInstances()))
	}
	if len(topo.GetConnections()) < 1 {
		t.Fatalf("want >=1 connection, got %d", len(topo.GetConnections()))
	}
}

func TestBake(t *testing.T) {
	e := New()
	form, err := e.InitialForm(context.Background(), "tenant", "demo", nil)
	if err != nil {
		t.Fatalf("InitialForm error: %v", err)
	}
	form, topo, errs, ready, err := e.Compute(context.Background(), "tenant", form)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if !ready || len(errs) != 0 {
		t.Fatalf("form not ready: ready=%v errs=%v", ready, errs)
	}

	draft := &models.TestWizardDraftRecord{
		Entity:   &common.Entity{Id: "draft-123", TenantId: "tenant", Name: "demo"},
		Form:     form,
		Topology: topo,
		Ready:    ready,
	}

	run, err := e.Bake(context.Background(), draft)
	if err != nil {
		t.Fatalf("Bake error: %v", err)
	}
	if run.GetId() == "" {
		t.Fatal("baked TestRun has empty id")
	}
	if run.GetProvider().GetProvider() != deployment.Provider_PROVIDER_DOCKER {
		t.Fatalf("want DOCKER provider, got %v", run.GetProvider().GetProvider())
	}
	if run.GetDatabase() == nil {
		t.Fatal("baked TestRun has no database")
	}
	if run.GetDatabase().GetKind() != 1 { // KIND_POSTGRES
		t.Fatalf("want KIND_POSTGRES database, got %v", run.GetDatabase().GetKind())
	}
	if run.GetDatabase().GetParams() == nil {
		t.Fatal("baked database has no params")
	}
	if run.GetWorkload() == nil {
		t.Fatal("baked TestRun has no workload")
	}
	if run.GetWorkload().GetStroppyVersion() != "v1" {
		t.Fatalf("want stroppy_version v1, got %q", run.GetWorkload().GetStroppyVersion())
	}
	if run.GetTopology() == nil || len(run.GetTopology().GetInstances()) < 1 {
		t.Fatal("baked TestRun has no topology instances")
	}

	if err := run.Validate(); err != nil {
		t.Fatalf("baked TestRun failed validation: %v", err)
	}
}

func TestBakeRejectsNotReady(t *testing.T) {
	e := New()
	// A draft with no form must be rejected.
	if _, err := e.Bake(context.Background(), &models.TestWizardDraftRecord{}); err == nil {
		t.Fatal("Bake should reject a draft with no form")
	}
}
