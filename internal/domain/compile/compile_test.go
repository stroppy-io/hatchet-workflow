package compile_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/compile"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/schemas"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// TestCompileCatalogTopologies compiles every catalog topology template of
// every deployable kind through the real schemas and bakes the result
// through spec.run@1: the RunSpec shape and every rendered config file are
// checked without a cloud.
func TestCompileCatalogTopologies(t *testing.T) {
	ctx := context.Background()
	reg := schemas.New()
	cat, err := catalog.New(ctx, reg, reg)
	if err != nil {
		t.Fatal(err)
	}
	lib := library.NewService(nil, reg, cat, nil, nil, nil, nil)
	provider, _ := cat.Provider("yandex")
	unsupported := map[string]bool{}

	for _, kind := range []catalog.DatabaseKind{catalog.Postgres, catalog.OrioleDB, catalog.MySQL, catalog.MariaDB, catalog.Cockroach, catalog.Picodata, catalog.YDB, catalog.PgNoop, catalog.Noop} {
		db, _ := cat.Database(kind)
		for _, topo := range db.Topologies {
			t.Run(string(kind)+"/"+topo.ID, func(t *testing.T) {
				params, _ := json.Marshal(topo.Params)
				dspec, derived, err := lib.DeriveDatabase(ctx, library.DatabaseSpec{Kind: kind, Version: defaultVersion(db), Params: params})
				if err != nil {
					if e, ok := errs.AsValidation(err); ok {
						t.Fatalf("derive: %v: %s", err, validationText(e))
					}
					t.Fatalf("derive: %v", err)
				}
				proto := db.Protocols[0]
				wspec, baked, _, err := lib.DeriveWorkload(ctx, library.WorkloadSpec{StroppyVersion: "6.0.0", Protocol: proto, Segments: []json.RawMessage{
					json.RawMessage(`{"name":"main","workload":{"script":"tpcc/tx","scale_factor":1},"run":{"vus":8,"duration":"1m"}}`),
				}})
				if err != nil {
					t.Fatalf("workload: %v", err)
				}
				sizes := map[string]library.RoleSize{}
				for _, n := range derived.Plan.Nodes {
					sizes[n.Role] = library.RoleSize{Size: "S"}
				}
				out, err := compile.Compile(ctx, reg, compile.Input{
					RunID: uuid.New(), Tenant: "acme", Database: dspec, Plan: derived.Plan, EffectiveConfigs: derived.EffectiveConfigs,
					Workload: wspec, WorkloadBaked: baked, Sizes: sizes, Provider: provider, ProviderKind: "yandex",
					ProviderSettings:  json.RawMessage(`{"cloud_id":"b1g","folder_id":"b1g","zone":"ru-central1-a"}`),
					CredentialsSecret: "provider-x", ProviderConfigName: "t-acme", Keep: time.Hour, Catalog: cat,
				})
				if err != nil {
					if errs.CodeOf(err) == errs.CodeInvalid && strings.Contains(err.Error(), "not launchable yet") {
						unsupported[string(kind)+"/"+topo.ID] = true
						t.Skipf("unsupported: %v", err)
					}
					if e, ok := errs.AsValidation(err); ok {
						t.Fatalf("compile: %v: %s", err, validationText(e))
					}
					t.Fatalf("compile: %v", err)
				}
				raw, err := json.Marshal(out.Spec)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := reg.Bake(ctx, "spec.run@1", raw); err != nil {
					if e, ok := errs.AsValidation(err); ok {
						t.Fatalf("spec.run@1: %s", validationText(e))
					}
					t.Fatalf("spec.run@1: %v", err)
				}
				if len(out.Spec.Machines) != derived.Plan.NodeCount() {
					t.Fatalf("machines %d != plan %d", len(out.Spec.Machines), derived.Plan.NodeCount())
				}
				if out.Spec.Workload.URL == "" || out.Spec.Workload.StroppyImage == "" {
					t.Fatalf("workload incomplete: %+v", out.Spec.Workload)
				}
				for _, ct := range out.Spec.Containers {
					if ct.Machine == "" || ct.Image == "" {
						t.Fatalf("container %s incomplete", ct.Name)
					}
				}
				t.Logf("%s: %d machines, %d containers, url %s", derived.Plan.Label, len(out.Spec.Machines), len(out.Spec.Containers), out.Spec.Workload.URL)
			})
		}
	}
	t.Logf("unsupported: %v", unsupported)
}

func validationText(e *errs.Error) string {
	b, _ := json.MarshalIndent(e.Validation, "", " ")
	return string(b)
}

func defaultVersion(db catalog.Database) string {
	for _, v := range db.Versions {
		if v.Default {
			return v.Version
		}
	}
	return db.Versions[0].Version
}

var _ = spec.Run{}
