package workload

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

const (
	groupScript    = "Script"
	groupExecution = "Execution"
	groupSizing    = "Sizing"
	groupFlags     = "Flags"
	groupData      = "Data loading"
	groupSteps     = "Steps"
	groupAdvanced  = "Advanced"
)

// durationPattern enforces a non-empty k6 duration ending in s/m/h, e.g. "60s",
// "10m", "1h" (mirrors old validate.go: suffix must be s, m or h). A leading
// number is required; bare suffixes ("s") are rejected.
const durationPattern = `^[0-9]+(\.[0-9]+)?[smh]$`

// workloadFields are the flat root fields of the PURE stroppy workload schema.
// schemapb has no nested sections, so fields are bucketed for the UI via
// Group(). Gating (When) honours the embed prefix through rp() exactly like the
// database schemas, so the schema stays embeddable under a parent.
func workloadFields(pfx string) []schemapb.FieldDef {
	return []schemapb.FieldDef{
		// --- Script selection ---
		schemapb.Str(FieldStroppyVersion).Required().MinLen(1).
			Default("").Group(groupScript).Title("Stroppy version").
			Desc("Stroppy binary version (semver, e.g. 5.1.1, or commit:<sha>)."),

		schemapb.Str(FieldScript).Required().MinLen(1).
			Default("tpcc/procs").Group(groupScript).Title("Script").
			Desc("Benchmark script, e.g. tpcc/procs, tpcc/tx, tpcb/procs, tpcb/tx, " +
				"or a path to a custom .ts / .sql."),

		schemapb.Str(FieldSQL).Default("").Group(groupScript).Title("SQL").
			Desc("Optional second stroppy positional arg (a .sql file name for SQL workloads)."),

		// --- k6 execution bounds ---
		schemapb.Str(FieldDuration).Default("60s").Pattern(durationPattern).
			Group(groupExecution).Title("Duration").
			Desc("k6 --duration; must end with s, m or h (e.g. 60s, 10m, 1h). " +
				"Used when k6_mode == duration.").
			When(utils.Eq(rp(pfx, FieldK6Mode), K6ModeDuration)),

		utils.StrEnum(FieldK6Mode, K6ModeValues...).Default(K6ModeDuration).
			Group(groupExecution).Title("Run mode").
			Desc("duration bounds the run by wall-clock; iterations by a fixed count."),

		schemapb.Int32(FieldIterations).Gte(0).Default(1).
			Group(groupExecution).Title("Iterations").
			Desc("k6 --iterations; used when k6_mode == iterations.").
			When(utils.Eq(rp(pfx, FieldK6Mode), K6ModeIterations)),

		schemapb.Int32(FieldVUs).Gte(0).Default(1).
			Group(groupExecution).Title("VUs").
			Desc("k6 --vus: number of virtual users."),

		// --- Sizing ---
		schemapb.Int32(FieldScaleFactor).Gte(0).Default(1).
			Group(groupSizing).Title("Scale factor").
			Desc("TPC-C warehouses / TPC-B branches → env SCALE_FACTOR."),

		schemapb.Int32(FieldPoolSize).Gte(0).Default(100).
			Group(groupSizing).Title("Pool size").
			Desc("DB connection pool size → env POOL_SIZE + driver pool."),

		// --- k6 flags ---
		schemapb.Bool(FieldQuiet).Default(true).
			Group(groupFlags).Title("Quiet").Desc("k6 -q flag. Enabled by default."),

		schemapb.Bool(FieldNoThresholds).Default(false).
			Group(groupFlags).Title("No thresholds").
			Desc("k6 --no-thresholds flag. Off by default."),

		// --- Data loading ---
		utils.StrEnum(FieldDefaultInsertMethod, InsertMethodValues...).Default(InsertNative).
			Group(groupData).Title("Default insert method").
			Desc("Driver defaultInsertMethod for generated data."),

		// --- Env overrides (List of {key,value}; schemapb has no map kind) ---
		schemapb.List(FieldEnv,
			schemapb.Object(FieldEntry,
				schemapb.Str(FieldKey).Required().MinLen(1),
				schemapb.Str(FieldValue).Required(),
			),
		).Group(groupData).Title("Environment overrides").
			Desc("Script-specific env var overrides (e.g. STROPPY_ERROR_MODE, LOAD_WORKERS)."),

		// --- Run-scoped files (List of {name,kind,content}) ---
		schemapb.List(FieldFiles,
			schemapb.Object(FieldFile,
				schemapb.Str(FieldName).Required().MinLen(1).Title("Name"),
				schemapb.Str(FieldKind).Default(FileKindSQL).Title("Kind").
					Desc("Currently sql; left open for future support files."),
				schemapb.Str(FieldContent).Required().Title("Content"),
			),
		).Group(groupData).Title("Files").
			Desc("Run-scoped files materialized beside stroppy-config.json."),

		// --- Steps allow/block ---
		schemapb.List(FieldSteps, schemapb.Str(FieldStep)).
			Group(groupSteps).Title("Steps").
			Desc("Step allowlist (e.g. create_schema, load_data, workload)."),

		schemapb.List(FieldNoSteps, schemapb.Str(FieldStep)).
			Group(groupSteps).Title("Skipped steps").
			Desc("Step blocklist (e.g. drop_schema)."),

		// --- Advanced escape hatch ---
		schemapb.Str(FieldConfigOverrideJSON).Default("").
			Group(groupAdvanced).Title("Config override JSON").
			Desc("Raw stroppy RunConfig protojson sent verbatim, bypassing the built config."),
	}
}
