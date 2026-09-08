// Package system holds the platform-level schemas: the stroppy catalog and
// size table an admin edits, the tenant limits, and the two small tenant-level
// forms (per-role sizes of a test, outgoing webhooks).
package system

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// protocolChoice is the wire protocol enum, identical to OpenAPI `Protocol`
// (openapi/parts/40-catalog.yaml).
func protocolChoice(name schemapb.FieldName) *schemapb.ChoiceB {
	return schemapb.Choice(name).
		Opt(schemapb.StrV("pg"), "PostgreSQL").
		Opt(schemapb.StrV("mysql"), "MySQL / MariaDB").
		Opt(schemapb.StrV("picodata"), "Picodata (pgproto)").
		Opt(schemapb.StrV("ydb_grpc"), "YDB (grpc)").
		Opt(schemapb.StrV("ydb_grpcs"), "YDB (grpcs, TLS)").
		Opt(schemapb.StrV("cockroach"), "CockroachDB (pgproto)").
		Opt(schemapb.StrV("noop"), "Noop (generator ceiling)")
}

// StroppyCatalog is system.stroppy_catalog@1 — which stroppy builds the
// platform offers, which scripts each one ships and what steps those scripts
// have. Edited by a platform admin in system settings; the workload form reads
// it, and the run compiler resolves `stroppy_version` to an image through it.
//
// The JSON shape matches OpenAPI StroppyCatalog / StroppyScript
// (openapi/parts/40-catalog.yaml, x-schema: system.stroppy_catalog).
//
// doc: STROPPY.MD §16.4; scripts and step ids verified against the stroppy
// checkout (README "Presets Tree", `stroppy help steps`).
func StroppyCatalog() *schemapb.Schema {
	return schemapb.NewSchema(ids.System("stroppy_catalog", 1)).
		Descr("Platform catalog of stroppy versions, their images, protocols and workload scripts.").
		Strict().Coerce().
		Fields(
			schemapb.Choice("source").Title("Source").Group("Catalog").
				Desc("Where the catalog came from: hand-written, read from a stroppy release, or probed.").
				Opt(schemapb.StrV("static"), "Static (admin-edited)").
				Opt(schemapb.StrV("release"), "Stroppy release").
				Opt(schemapb.StrV("probe"), "Probed from a binary").
				Default(schemapb.StrV("static")),

			schemapb.List("versions",
				schemapb.Object("",
					// doc: semver.org
					schemapb.Str("version").Title("Version").
						Desc("Stroppy release version.").
						Pattern(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`).MaxLen(64).Required(),
					schemapb.Str("image").Title("Image").
						Desc("Docker image of that release, e.g. ghcr.io/stroppy-io/stroppy:5.1.2.").
						MinLen(1).MaxLen(512).Required(),
					schemapb.Bool("default").Title("Default").
						Desc("The version a new workload starts with; exactly one version carries it.").
						Default(false),
					schemapb.Bool("deprecated").Title("Deprecated").
						Desc("Still runnable, hidden from the picker for new workloads.").
						Default(false),
					schemapb.List("protocols", protocolChoice("")).
						Title("Protocols").
						Desc("Database protocols this build has drivers for.").
						MinItems(1).MaxItems(16).Unique().Required(),

					schemapb.List("scripts",
						schemapb.Object("",
							// doc: stroppy README "Presets Tree" — ids look like
							// tpcc/tx, tpcb/procs, tpch/tx, tpcds, simple.
							schemapb.Str("id").Title("Script id").
								Desc("Workload script id as passed to `stroppy run`.").
								Pattern(`^[a-z0-9_]+(/[a-z0-9_]+)*$`).MaxLen(128).Required().
								Examples(schemapb.StrV("tpcc/tx"), schemapb.StrV("tpch/tx")),
							schemapb.Str("title").Title("Title").MinLen(1).MaxLen(128).Required(),
							schemapb.Str("description").Title("Description").MaxLen(2048),
							schemapb.List("protocols", protocolChoice("")).
								Title("Protocols").
								Desc("Protocols this script runs on; procs variants are pg/mysql only.").
								MaxItems(16).Unique(),
							schemapb.List("steps",
								schemapb.Object("",
									// doc: stroppy `help steps` — drop_schema,
									// create_schema, load_data, and the workload steps.
									schemapb.Str("id").Title("Step id").
										Pattern(`^[a-z][a-z0-9_]*$`).MaxLen(64).Required(),
									schemapb.Str("title").Title("Title").MaxLen(128),
									schemapb.Choice("phase").Title("Phase").
										Desc("Where the step sits in a run: preparing, measuring, cleaning up.").
										Opt(schemapb.StrV("bootstrap"), "Bootstrap").
										Opt(schemapb.StrV("workload"), "Workload").
										Opt(schemapb.StrV("teardown"), "Teardown").
										Default(schemapb.StrV("workload")),
								).Strict(),
							).Title("Steps").
								Desc("Steps the script declares; the segment form filters on them.").
								MaxItems(64).Required(),
							schemapb.Str("params_schema").Title("Params schema").
								Desc("schemapb id of script-specific params, when the script has its own form.").
								Pattern(`^[a-z][a-z0-9_.]*@\d+$`).MaxLen(128),
						).Strict(),
					).Title("Scripts").
						Desc("Workload scripts this build ships.").
						MinItems(1).MaxItems(128).Required(),
				).Strict(),
			).Title("Versions").Group("Catalog").
				Desc("Every stroppy build the platform offers.").
				MinItems(1).MaxItems(64).Required(),
		).
		Rules(schemapb.Rule(
			`!("versions" in root) || size(root.versions.filter(v, v.default)) == 1`,
			"exactly one version must be the default",
		).ID("exactly-one-default")).
		MustBuild()
}
