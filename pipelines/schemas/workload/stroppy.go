package workload

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// segmentDef is the local $defs key the segment schema is registered under.
const segmentDef = "segment"

// Stroppy is workload.stroppy@1 — the root of a Workload library record: which
// stroppy build runs, over which wire protocol, with which segments and
// driver options.
//
// doc: OpenAPI Protocol enum (openapi/parts/40-catalog.yaml) — the JSON shape
// of `protocol` must stay identical on both doors.
func Stroppy() *schemapb.Schema {
	return schemapb.NewSchema(ids.Workload("stroppy", 1)).
		Descr("A stroppy workload: version, protocol, load segments and driver options.").
		Strict().Coerce().
		DefSchema(segmentDef, Segment()).
		Fields(
			// The set of usable versions comes from system.stroppy_catalog@1;
			// here only the shape is checked.
			// doc: semver.org — MAJOR.MINOR.PATCH with optional pre-release/build.
			schemapb.Str("stroppy_version").Title("Stroppy version").Group("Workload").
				Desc("Stroppy release to run; must exist in the platform stroppy catalog.").
				Pattern(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`).MaxLen(64).Required().
				Examples(schemapb.StrV("5.1.2")),

			// doc: stroppy `help drivers` — driverType postgres | mysql |
			// picodata | ydb | noop | csv. The product-level protocol splits ydb
			// by transport (grpc/grpcs) and adds cockroach, which speaks the
			// postgres wire protocol.
			schemapb.Choice("protocol").Title("Protocol").Group("Workload").
				Desc("Wire protocol stroppy talks to the database with.").
				Opt(schemapb.StrV("pg"), "PostgreSQL").
				Opt(schemapb.StrV("mysql"), "MySQL / MariaDB").
				Opt(schemapb.StrV("picodata"), "Picodata (pgproto)").
				Opt(schemapb.StrV("ydb_grpc"), "YDB (grpc)").
				Opt(schemapb.StrV("ydb_grpcs"), "YDB (grpcs, TLS)").
				Opt(schemapb.StrV("cockroach"), "CockroachDB (pgproto)").
				Opt(schemapb.StrV("noop"), "Noop (generator ceiling)").
				Required(),

			schemapb.List("segments",
				schemapb.Ref("", segmentDef),
			).Title("Segments").Group("Segments").
				Desc("Ordered stroppy invocations; each one is a phase of the run.").
				MinItems(1).MaxItems(64).Required(),

			// doc: stroppy `help drivers` — the -D option keys per driver type.
			schemapb.OneOf("driver_options", "kind").Title("Driver options").Group("Driver").
				Desc("Protocol-specific connection options; the kind must match the protocol.").
				Variant("pg",
					// doc: postgresql.org/docs/current/libpq-ssl.html — libpq also
					// defines allow/prefer/verify-ca; the product exposes the three
					// that are meaningful for a benchmark.
					schemapb.Choice("sslmode").Title("SSL mode").
						Desc("TLS negotiation mode of the postgres connection.").
						Opt(schemapb.StrV("disable"), "Disable").
						Opt(schemapb.StrV("require"), "Require").
						Opt(schemapb.StrV("verify-full"), "Verify full").
						Default(schemapb.StrV("disable")),
					schemapb.Str("application_name").Title("Application name").
						Desc("application_name reported to postgres; shows up in pg_stat_activity.").
						MaxLen(64).Default("stroppy"),
					// doc: pkg.go.dev/github.com/jackc/pgx/v5#QueryExecMode
					schemapb.Choice("statement_cache").Title("Statement cache").
						Desc("pgx query execution mode: how prepared statements are cached.").
						Opt(schemapb.StrV("cache_statement"), "Cache prepared statements").
						Opt(schemapb.StrV("cache_describe"), "Cache statement descriptions").
						Opt(schemapb.StrV("describe_exec"), "Describe then exec").
						Opt(schemapb.StrV("exec"), "Exec (no cache)").
						Opt(schemapb.StrV("simple_protocol"), "Simple protocol").
						Default(schemapb.StrV("cache_statement")),
				).
				Variant("mysql",
					// doc: github.com/go-sql-driver/mysql#tls — true | false |
					// skip-verify | preferred | <custom config name>.
					schemapb.Choice("tls").Title("TLS").
						Desc("go-sql-driver TLS mode of the MySQL connection.").
						Opt(schemapb.StrV("false"), "Off").
						Opt(schemapb.StrV("preferred"), "Preferred").
						Opt(schemapb.StrV("skip-verify"), "On, no verification").
						Opt(schemapb.StrV("true"), "On, verified").
						Default(schemapb.StrV("false")),
					// doc: dev.mysql.com/doc/refman/8.4/en/charset-charsets.html
					schemapb.Str("charset").Title("Charset").
						Desc("Connection character set.").
						Pattern(`^[a-z0-9_]+$`).MaxLen(32).Default("utf8mb4"),
				).
				Variant("ydb",
					// doc: stroppy `help drivers` — grpcs:// URL turns TLS on;
					// caCertFile / authToken are the private-CA and IAM knobs.
					schemapb.Bool("grpcs").Title("TLS (grpcs)").
						Desc("Use the grpcs:// scheme; must agree with the protocol.").Default(false),
					schemapb.Str("ca_cert").Title("CA certificate").
						Desc("PEM of a private CA, when the endpoint is not signed by a public one.").
						MaxLen(1<<16).Secret(),
					schemapb.Str("token").Title("Auth token").
						Desc("IAM token passed as authToken.").MaxLen(4096).Secret(),
				).
				Variant("picodata").
				Variant("cockroach",
					// doc: cockroachlabs.com/docs/stable/connection-parameters — the
					// pgwire sslmode values CockroachDB honors.
					schemapb.Choice("sslmode").Title("SSL mode").
						Desc("TLS negotiation mode of the cockroach (pgwire) connection.").
						Opt(schemapb.StrV("disable"), "Disable").
						Opt(schemapb.StrV("require"), "Require").
						Opt(schemapb.StrV("verify-full"), "Verify full").
						Default(schemapb.StrV("disable")),
				).
				Variant("noop"),
		).
		Rules(
			schemapb.Rule(
				`!("driver_options" in root) || !("protocol" in root) || `+
					`root.driver_options.kind == (root.protocol.startsWith("ydb") ? "ydb" : root.protocol)`,
				"driver_options.kind must match the protocol",
			).ID("driver-options-match-protocol"),
			schemapb.Rule(
				`size(root.segments.map(s, s.name)) == size(root.segments.map(s, s.name).filter(`+
					`n, root.segments.filter(x, x.name == n).size() == 1))`,
				"segment names must be unique",
			).ID("segment-names-unique"),
		).
		MustBuild()
}
