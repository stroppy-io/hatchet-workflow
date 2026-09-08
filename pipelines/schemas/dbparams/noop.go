package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Noop is db.noop.params@1 — no database at all: stroppy's internal noop
// driver, used to measure the generator's own ceiling on a machine.
func Noop() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("noop", 1)).
		Descr("No database — stroppy noop driver; measures the load generator ceiling.").
		Strict().Coerce().
		Fields(
			schemapb.Int64("workers").Title("Workers").Group("Generator").
				Desc("Parallel noop workers inside stroppy; 0 = one per CPU of the runner.").
				Gte(0).Lte(4096).Default(0),
		).
		MustBuild()
}
