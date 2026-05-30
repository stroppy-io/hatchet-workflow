package workload

import "github.com/stroppy-io/stroppy-cloud/internal/schemas/types"

// String vocabulary for the PURE stroppy workload schema: enum values, field
// names, and the expr paths used by its (workload-internal) gates. This schema
// is provider-agnostic AND db-agnostic — it knows nothing about database kinds,
// protocols, machines or providers. Script↔kind and protocol↔kind compatibility
// is a cross-schema wizard concern and lives nowhere in here.
//
// Enum value types are string aliases so they pass straight into
// utils.StrEnum(...).In() / .Default(); each ships a <Name>Values slice.

// =============================================================================
// Enum value types (string) + values
// =============================================================================

// K6Mode selects how k6 bounds the run: by wall-clock duration or by a fixed
// iteration count. The iterations field is only meaningful (and only shown by
// the FE) when k6_mode == iterations.
type K6Mode = string

const (
	K6ModeDuration   K6Mode = "duration"
	K6ModeIterations K6Mode = "iterations"
)

var K6ModeValues = []string{K6ModeDuration, K6ModeIterations}

// InsertMethod is the driver's data-loading insert path (defaultInsertMethod).
type InsertMethod = string

const (
	InsertNative     InsertMethod = "native"
	InsertPlainBulk  InsertMethod = "plain_bulk"
	InsertPlainQuery InsertMethod = "plain_query"
)

var InsertMethodValues = []string{InsertNative, InsertPlainBulk, InsertPlainQuery}

// FileKind is the kind of a run-scoped workload file. Currently only "sql" is
// recognised by the FE; left open (no In()) for future script/support files.
type FileKind = string

const (
	FileKindSQL FileKind = "sql"
)

var FileKindValues = []string{FileKindSQL}

// =============================================================================
// Field names
// =============================================================================

const (
	// root — identity / script selection
	FieldStroppyVersion = types.Field("stroppy_version")
	FieldScript         = types.Field("script")
	FieldSQL            = types.Field("sql")

	// root — k6 execution bounds
	FieldDuration   = types.Field("duration")
	FieldK6Mode     = types.Field("k6_mode")
	FieldIterations = types.Field("iterations")
	FieldVUs        = types.Field("vus")

	// root — workload sizing
	FieldPoolSize    = types.Field("pool_size")
	FieldScaleFactor = types.Field("scale_factor")

	// root — k6 flags
	FieldQuiet        = types.Field("quiet")
	FieldNoThresholds = types.Field("no_thresholds")

	// root — driver / data loading
	FieldDefaultInsertMethod = types.Field("default_insert_method")

	// root — env overrides (List of {key,value} — schemapb has no map kind)
	FieldEnv   = types.Field("env")
	FieldEntry = types.Field("entry")
	FieldKey   = types.Field("key")
	FieldValue = types.Field("value")

	// root — run-scoped files (List of {name,kind,content})
	FieldFiles   = types.Field("files")
	FieldFile    = types.Field("file")
	FieldName    = types.Field("name")
	FieldKind    = types.Field("kind")
	FieldContent = types.Field("content")

	// root — step allow/block lists (List of str)
	FieldSteps   = types.Field("steps")
	FieldNoSteps = types.Field("no_steps")
	FieldStep    = types.Field("step")

	// root — escape hatch
	FieldConfigOverrideJSON = types.Field("config_override_json")
)
