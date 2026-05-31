// Package testengine implements the test_wizard.WizardEngine for the demo: it
// composes a single conditional form (postgres database + stroppy workload +
// provider_type selector), validates the submitted values, derives the
// deployment topology from the database config (role->VM expander + recipe
// wiring) and bakes a ready draft into a materialized domain.TestRun.
//
// The engine is PURE: it performs no IO and uses no time/rand, so it is safe to
// run inside the service's retried serializable transaction. Identifiers that
// must be stable are derived deterministically from the draft id.
package testengine

import (
	"context"
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stroppy-io/stroppy-cloud/internal/deploy/recipe"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/databases/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/expand"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/workload"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_wizard"
)

// Field keys of the composite wizard form.
const (
	fieldDatabase     = "database"
	fieldWorkload     = "workload"
	fieldProviderType = "provider_type"

	// dbKind is the only database engine this demo engine handles.
	dbKind = "postgres"

	// providerDocker is the default (and only) provider for the demo.
	providerDocker = "docker"

	// workloadRunners is the number of stroppy runner VMs appended to the
	// database VMs.
	workloadRunners = 1
)

// Engine is the demo WizardEngine: postgres + workload + docker.
type Engine struct{}

// New constructs the demo wizard engine.
func New() *Engine { return &Engine{} }

var _ test_wizard.WizardEngine = (*Engine)(nil)

// compositeSchema builds the whole test form as ONE schema: the postgres
// database schema (embedded under "database."), the stroppy workload schema
// (embedded under "workload.") and a provider_type selector. The embedded
// schemas take the matching root prefix so their internal When gates resolve.
func compositeSchema() *schemapb.Schema {
	return schemapb.NewSchema("wizard", "test", "1.0.0").
		Descr("Composite test wizard form: database + workload + provider selector.").
		Fields(
			schemapb.ObjectOf(fieldDatabase, postgres.PostgresSchema(fieldDatabase+".")),
			schemapb.ObjectOf(fieldWorkload, workload.WorkloadSchema(fieldWorkload+".")),
			schemapb.Str(fieldProviderType).Default(providerDocker),
		).
		MustBuild()
}

// schemaRef wraps the composite schema as an inline SchemaRef for a Filled.
func schemaRef() *schemapb.SchemaRef {
	return &schemapb.SchemaRef{
		Source: &schemapb.SchemaRef_Schema{Schema: compositeSchema()},
	}
}

// defaultValues are the starting form values: a single-node postgres, a minimal
// stroppy workload and the docker provider.
func defaultValues() *structpb.Struct {
	st, err := structpb.NewStruct(map[string]any{
		fieldDatabase: map[string]any{
			string(postgres.FieldTopology): postgres.TopologySingle,
			string(postgres.FieldPgMajor):  float64(postgres.PgMajor16),
		},
		fieldWorkload: map[string]any{
			string(workload.FieldStroppyVersion): "v1",
			string(workload.FieldScript):         "tpcc/procs",
			string(workload.FieldDuration):       "60s",
			string(workload.FieldVUs):            float64(1),
		},
		fieldProviderType: providerDocker,
	})
	if err != nil {
		// The literal above is always valid structpb input; a failure here is a
		// programming error.
		panic(fmt.Errorf("testengine: build default values: %w", err))
	}
	return st
}

// InitialForm returns the starting composite form (schema + default values).
// The seed is ignored for the demo: a fresh single-node postgres form is always
// returned.
func (e *Engine) InitialForm(_ context.Context, _ /*tenantID*/, _ /*name*/ string, _ *models.TestPresetRecord) (*schemapb.Filled, error) {
	return &schemapb.Filled{
		Schema: schemaRef(),
		Values: defaultValues(),
	}, nil
}

// Compute validates the submitted form against the composite schema, derives the
// topology from the database sub-values and reports readiness. It is pure: same
// form in => same result out.
func (e *Engine) Compute(_ context.Context, _ string, form *schemapb.Filled) (*schemapb.Filled, *topology.Topology, []*schemapb.FieldError, bool, error) {
	if form == nil {
		form = &schemapb.Filled{Schema: schemaRef(), Values: defaultValues()}
	}
	if form.GetValues() == nil {
		form.Values = defaultValues()
	}
	// Always re-emit the canonical composite schema so the form is self-describing
	// regardless of what the client sent.
	form.Schema = schemaRef()

	schema := compositeSchema()
	errs := schema.ValidateStruct(form.GetValues())

	topo, terr := buildTopology(form.GetValues())
	if terr != nil {
		errs = append(errs, schemapb.NewFieldError(fieldDatabase, terr.Error()))
	}

	ready := len(errs) == 0
	return form, topo, errs, ready, nil
}

// dbValues extracts the database sub-map from the composite form values.
func dbValues(values *structpb.Struct) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	all := values.AsMap()
	if sub, ok := all[fieldDatabase].(map[string]any); ok {
		return sub
	}
	return map[string]any{}
}

// workloadValues extracts the workload sub-map from the composite form values.
func workloadValues(values *structpb.Struct) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	all := values.AsMap()
	if sub, ok := all[fieldWorkload].(map[string]any); ok {
		return sub
	}
	return map[string]any{}
}

// buildTopology derives the deployment topology from the database config: the
// role->VM expander turns the database config into machines (+ a stroppy runner
// VM), each VM becomes a Topology_Instance and the recipe wires the connections.
func buildTopology(values *structpb.Struct) (*topology.Topology, error) {
	db := dbValues(values)

	dbVMs, err := expand.Expand(dbKind, db)
	if err != nil {
		return nil, err
	}
	vms := expand.WithWorkload(dbVMs, workloadRunners)

	instances := make([]*topology.Topology_Instance, 0, len(vms))
	rolesToIDs := map[string][]string{}
	for _, vm := range vms {
		// The component id is kept equal to the instance/vm id so the
		// connections produced by recipe.Wire (which keys on vm names) resolve
		// against these component ids.
		instanceID := vm.Name
		instances = append(instances, &topology.Topology_Instance{
			Id: instanceID,
			MachineInfo: &deployment.MachineInfo{
				Cores:    uint32(vm.Shape.Cores),
				MemoryGb: uint64(vm.Shape.MemoryGB),
				DiskGb:   uint64(vm.Shape.DiskGB),
			},
			Components: []*topology.Component{{
				Id:                    instanceID,
				Kind:                  roleToKind(vm.Role),
				AllocatedOnInstanceId: &instanceID,
			}},
		})
		rolesToIDs[string(vm.Role)] = append(rolesToIDs[string(vm.Role)], vm.Name)
	}

	conns := recipe.Wire(dbKind, rolesToIDs)

	return &topology.Topology{
		Instances:   instances,
		Connections: conns,
	}, nil
}

// roleToKind maps an expand VM role to the topology Component kind hosted on the
// instance. Unknown roles fall back to KIND_ADDON.
func roleToKind(role expand.Role) topology.Component_Kind {
	switch role {
	case expand.RoleDatabase:
		return topology.Component_KIND_DATABASE
	case expand.RoleReplica:
		return topology.Component_KIND_REPLICA
	case expand.RoleWorkload:
		return topology.Component_KIND_WORKLOAD
	case expand.RoleCoordinator:
		return topology.Component_KIND_COORDINATOR
	case expand.RoleProxy:
		return topology.Component_KIND_PROXY
	default:
		return topology.Component_KIND_ADDON
	}
}

// Bake turns a ready draft into a materialized domain.TestRun: the database
// params, the workload, the docker provider settings and the derived topology.
// It re-validates the persisted form and rejects a draft that is not ready.
func (e *Engine) Bake(_ context.Context, draft *models.TestWizardDraftRecord) (*domain.TestRun, error) {
	if draft == nil || draft.GetForm() == nil {
		return nil, fmt.Errorf("testengine: bake: draft has no form")
	}
	form := draft.GetForm()

	schema := compositeSchema()
	if errs := schema.ValidateStruct(form.GetValues()); len(errs) > 0 {
		return nil, fmt.Errorf("testengine: bake: draft is not ready: %s", errs[0].GetMessage())
	}

	topo, err := buildTopology(form.GetValues())
	if err != nil {
		return nil, fmt.Errorf("testengine: bake: build topology: %w", err)
	}

	dbBaked, err := bakedSub(dbValues(form.GetValues()), postgres.PostgresSchema(""))
	if err != nil {
		return nil, fmt.Errorf("testengine: bake: database params: %w", err)
	}
	wlValues := workloadValues(form.GetValues())
	wlBaked, err := bakedSub(wlValues, workload.WorkloadSchema(""))
	if err != nil {
		return nil, fmt.Errorf("testengine: bake: workload params: %w", err)
	}

	database := &domain.Database{
		Kind:        domain.Database_KIND_POSTGRES,
		PramsSchema: postgres.PostgresSchema(""),
		Source:      &domain.Database_Params{Params: dbBaked},
	}

	stroppyVersion, _ := wlValues[string(workload.FieldStroppyVersion)].(string)
	workloadMsg := &domain.Workload{
		StroppyVersion: stroppyVersion,
		Params:         wlBaked,
	}

	provider := &deployment.ProviderSettings{
		Provider: providerEnum(form.GetValues()),
	}

	return &domain.TestRun{
		Id:       runID(draft),
		Provider: provider,
		Topology: topo,
		Database: database,
		Workload: workloadMsg,
	}, nil
}

// bakedSub wraps a sub-map of values as a schemapb.Baked against the given
// (standalone) schema.
func bakedSub(values map[string]any, schema *schemapb.Schema) (*schemapb.Baked, error) {
	st, err := structpb.NewStruct(values)
	if err != nil {
		return nil, err
	}
	return &schemapb.Baked{Schema: schema, Values: st}, nil
}

// providerEnum maps the form's provider_type selector to the deployment enum.
// The demo only supports docker.
func providerEnum(values *structpb.Struct) deployment.Provider {
	if values != nil {
		if pt, ok := values.AsMap()[fieldProviderType].(string); ok {
			switch pt {
			case providerDocker:
				return deployment.Provider_PROVIDER_DOCKER
			}
		}
	}
	return deployment.Provider_PROVIDER_DOCKER
}

// runID derives a stable test-run id from the draft id (no time/rand so a
// retried bake produces the same id). Falls back to a constant when the draft
// carries no id.
func runID(draft *models.TestWizardDraftRecord) string {
	id := draft.GetEntity().GetId()
	if id == "" {
		return "test-run"
	}
	return "run-" + id
}
