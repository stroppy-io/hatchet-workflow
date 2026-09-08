package system

import (
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// sizeChoice is the T-shirt size picker shared by the limits and the per-role
// sizes of a test.
// doc: OpenAPI `Size` (openapi/parts/10-components-common.yaml).
func sizeChoice(name schemapb.FieldName) *schemapb.ChoiceB {
	return schemapb.Choice(name).
		Opt(schemapb.StrV("XS"), "XS").
		Opt(schemapb.StrV("S"), "S").
		Opt(schemapb.StrV("M"), "M").
		Opt(schemapb.StrV("L"), "L").
		Opt(schemapb.StrV("XL"), "XL")
}

// Limits is system.limits@1 — the ceiling a tenant may spend, set as a
// platform default and overridable per tenant by an admin.
//
// The JSON shape matches OpenAPI TenantLimits
// (openapi/parts/30-tenant-settings.yaml) minus the read-only `source`.
//
// doc: STROPPY.MD §16.3, §16.9.
func Limits() *schemapb.Schema {
	return schemapb.NewSchema(ids.System("limits", 1)).
		Descr("Tenant limits: how much a tenant may run at once, how big, and for how long.").
		Strict().Coerce().
		Fields(
			schemapb.Int64("max_concurrent_runs").Title("Concurrent runs").Group("Limits").
				Desc("Runs a tenant may have in flight, suite children included.").
				Gte(1).Lte(1000).Default(3).Required(),
			schemapb.Int64("max_machines_per_run").Title("Machines per run").Group("Limits").
				Desc("Upper bound on the machine count of one RunSpec.").
				Gte(1).Lte(64).Default(8).Required(),
			sizeChoice("max_size").Title("Largest size").Group("Limits").
				Desc("Largest T-shirt size any role of a run may use.").
				Default(schemapb.StrV("L")).Required(),
			schemapb.Duration("max_keep").Title("Longest stand").Group("Limits").
				Desc("Longest a stand may be kept alive after a run finishes.").
				Gte(0).Lte(30*24*time.Hour).Default(24*time.Hour).Required(),
			schemapb.Int64("run_retention_max_days").Title("Run retention").Unit("days").Group("Limits").
				Desc("Ceiling for the tenant's own run_retention_days setting.").
				Gte(1).Lte(3650).Default(180).Required(),
		).
		MustBuild()
}
