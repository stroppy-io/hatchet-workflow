// Package workload holds the schemas of a stroppy workload: one load segment
// and the whole workload record (protocol, version, segments, driver options).
package workload

import (
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// stepPattern is a stroppy step id as printed by `stroppy probe <script>
// --steps` (drop_schema, create_schema, load_data, ...).
// doc: stroppy cmd/stroppy/commands/help/topic_steps.go
const stepPattern = `^[a-z][a-z0-9_]*$`

// envKeyPattern is the shell-style env key stroppy accepts with `-e KEY=VALUE`
// (keys are upper-cased by the CLI).
// doc: stroppy cmd/stroppy/commands/help/topic_envs.go
const envKeyPattern = `^[A-Z_][A-Z0-9_]*$`

// Segment is workload.segment@1 — one stroppy invocation: which script runs,
// how long, with what data-generation parameters and side files. A Workload is
// an ordered list of these.
func Segment() *schemapb.Schema {
	return schemapb.NewSchema(ids.Workload("segment", 1)).
		Descr("One stroppy load segment: script, execution limits, generator parameters and files.").
		Strict().Coerce().
		Fields(
			schemapb.Str("name").Title("Name").Group("Segment").
				Desc("Segment id inside the workload; used as the phase label of the run and as a metric label.").
				Pattern(`^[a-z][a-z0-9_-]*$`).MinLen(1).MaxLen(64).Required(),

			// The catalog (system.stroppy_catalog@1) restricts the value to the
			// scripts the chosen stroppy version ships; the schema only checks shape.
			// doc: stroppy README "Presets Tree" (tpcc/tx, tpcb/procs, tpch/tx, tpcds)
			schemapb.Str("script").Title("Script").Group("Segment").
				Desc("Stroppy workload script id or path, e.g. tpcc/tx. Restricted to the catalog by the server.").
				MinLen(1).MaxLen(256).Required().
				Examples(schemapb.StrV("tpcc/tx"), schemapb.StrV("tpch/tx"), schemapb.StrV("simple")),

			// doc: k6 options behind `stroppy run … -- --vus --duration --iterations`
			schemapb.Object("execution",
				schemapb.Int64("vus").Title("Virtual users").Group("Execution").
					Desc("k6 virtual users driving the segment; sizes the runner machine.").
					Gte(1).Lte(100000).Default(1),
				schemapb.OneOf("limit", "kind").Title("Limit").Group("Execution").
					Desc("How the segment ends: after a wall-clock duration or after N iterations.").
					Variant("duration",
						schemapb.Duration("duration").Title("Duration").
							Desc("Wall-clock length of the segment (k6 --duration).").
							Gt(0).Lte(24*time.Hour).Required(),
					).
					Variant("iterations",
						schemapb.Int64("count").Title("Iterations").
							Desc("Total iterations across all VUs (k6 --iterations).").
							Gte(1).Required(),
					).Required(),
				schemapb.Bool("quiet").Title("Quiet").Group("Execution").
					Desc("Suppress the k6 progress bar (k6 --quiet).").Default(true),
				schemapb.Bool("no_thresholds").Title("Ignore thresholds").Group("Execution").
					Desc("Do not fail the segment on threshold breach (k6 --no-thresholds).").Default(false),
				schemapb.List("extra_args",
					schemapb.Str("").MinLen(1).MaxLen(256),
				).Title("Extra k6 args").Group("Execution").
					Desc("Raw arguments appended after `--` to the embedded k6 process.").
					MaxItems(64),
			).Title("Execution").Group("Execution").
				Desc("Load shape of the segment.").Strict().Required(),

			schemapb.Object("params",
				schemapb.Int64("pool_size").Title("Pool size").Group("Parameters").
					Desc("Driver connection pool size (POOL_SIZE / pool.maxConns).").
					Gte(1).Lte(65535),
				schemapb.Int64("scale_factor").Title("Scale factor").Group("Parameters").
					Desc("Dataset scale (warehouses for TPC-C, SF for TPC-H); drives the disk requirement.").
					Gte(1).Lte(100000).Default(1),
				schemapb.List("steps",
					schemapb.Str("").Pattern(stepPattern).MinLen(1).MaxLen(64),
				).Title("Only these steps").Group("Parameters").
					Desc("Run only the listed stroppy steps (`--steps`); empty = all steps.").
					MaxItems(32).Unique(),
				schemapb.List("no_steps",
					schemapb.Str("").Pattern(stepPattern).MinLen(1).MaxLen(64),
				).Title("Skip these steps").Group("Parameters").
					Desc("Skip the listed stroppy steps (`--no-steps`); mutually exclusive with steps.").
					MaxItems(32).Unique(),
				// doc: stroppy `help drivers` — defaultInsertMethod is one of
				// plain_query | native | plain_bulk (verified in the stroppy checkout,
				// cmd/stroppy/commands/help/topic_drivers.go). The main-v0 wording
				// "batch|copy|multirow" was never a stroppy value.
				schemapb.Choice("insert_method").Title("Insert method").Group("Parameters").
					Desc("Driver-level write protocol for generated rows (defaultInsertMethod).").
					Opt(schemapb.StrV("native"), "Native (CopyFrom / BulkUpsert)").
					Opt(schemapb.StrV("plain_bulk"), "Plain bulk INSERT").
					Opt(schemapb.StrV("plain_query"), "Plain single-row INSERT"),
				// doc: stroppy `help drivers` — bulkSize default 2500.
				schemapb.Int64("bulk_size").Title("Bulk size").Group("Parameters").
					Desc("Rows per bulk INSERT statement; only used by plain_bulk.").
					Unit("rows").Gte(1).Lte(1000000).Default(2500),
				schemapb.MapOf("env", schemapb.Str("value").MaxLen(4096)).
					Title("Environment").Group("Parameters").
					Desc("Extra script environment (`-e KEY=VALUE`); keys must match "+envKeyPattern+".").
					MaxEntries(256).
					Rules(schemapb.Rule(
						`this.all(k, k.matches("`+envKeyPattern+`"))`,
						"env keys must be upper-case identifiers",
					).ID("env-key-shape")),
			).Title("Parameters").Group("Parameters").
				Desc("Data-generation and driver parameters passed to the script.").Strict().
				Rule(
					schemapb.Rule(
						`!("steps" in this) || !("no_steps" in this) || this.steps.all(s, !(s in this.no_steps))`,
						"steps and no_steps must not overlap",
					).ID("steps-disjoint"),
					// doc: stroppy cmd/stroppy/commands/run/run.go — errStepsMutExclusive.
					schemapb.Rule(
						`!("steps" in this) || !("no_steps" in this) || size(this.steps) == 0 || size(this.no_steps) == 0`,
						"stroppy rejects --steps together with --no-steps",
					).ID("steps-mutually-exclusive").Warn(),
				),

			schemapb.List("files",
				schemapb.Object("",
					schemapb.Str("name").Title("File name").
						Desc("Name the file gets next to the script on the runner.").
						Pattern(`^[A-Za-z0-9._/-]+$`).MinLen(1).MaxLen(128).Required(),
					schemapb.Choice("kind").Title("Kind").
						Desc("What the file is: a schema/DDL file, a config, or a data file.").
						Opt(schemapb.StrV("sql"), "SQL / DDL").
						Opt(schemapb.StrV("conf"), "Config").
						Opt(schemapb.StrV("data"), "Data").
						Default(schemapb.StrV("sql")),
					schemapb.Str("content").Title("Content").
						Desc("Inline file body, at most 1 MiB.").MaxLen(1<<20),
					schemapb.Str("ref").Title("Reference").
						Desc("Artifact/object reference to fetch instead of inline content.").
						MinLen(1).MaxLen(512),
				).Strict().Rule(schemapb.Rule(
					`("content" in this) != ("ref" in this)`,
					"a file needs exactly one of content or ref",
				).ID("file-content-xor-ref")),
			).Title("Files").Group("Files").
				Desc("Extra files (SQL schema, configs, data) shipped with the segment.").
				MaxItems(32),

			schemapb.Object("thresholds",
				schemapb.Double("p99_ms").Title("p99 latency").Unit("ms").
					Desc("Fail the segment when the p99 request latency exceeds this.").Gt(0),
				schemapb.Double("error_rate").Title("Error rate").Unit("ratio").
					Desc("Fail the segment when the error ratio exceeds this (0..1).").Gte(0).Lte(1),
			).Title("Thresholds").Group("Thresholds").
				Desc("Pass/fail thresholds handed to k6; ignored when no_thresholds is set.").Strict(),

			schemapb.Duration("warmup").Title("Warm-up").Group("Execution").
				Desc("Idle wait before the segment starts, letting caches and replicas settle.").
				Gte(0).Lte(time.Hour).Default(0),
		).
		MustBuild()
}
