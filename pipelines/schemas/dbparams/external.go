package dbparams

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// External is db.external.params@1 — a database the product does not own. No
// machines are provisioned and no software is deployed: the run skips straight
// from provisioning to the workload, pointed at a DSN the tenant supplies.
func External() *schemapb.Schema {
	return schemapb.NewSchema(ids.DB("external", 1)).
		Descr("An already-running database addressed by DSN; the stand provisions and deploys nothing.").
		Strict().Coerce().
		Fields(
			// The protocol decides which stroppy driver is used and therefore
			// which DSN shape is legal; keeping it explicit avoids guessing it
			// out of the URI scheme.
			schemapb.Choice("protocol").Title("Protocol").Group("Connection").
				Desc("Wire protocol stroppy speaks to this database.").
				// doc: https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
				Opt(schemapb.StrV("pg"), "PostgreSQL wire protocol").
				// doc: https://dev.mysql.com/doc/refman/8.4/en/connecting-using-uri-or-key-value-pairs.html
				Opt(schemapb.StrV("mysql"), "MySQL / MariaDB").
				// doc: https://docs.picodata.io/picodata/stable/tutorial/connecting/
				Opt(schemapb.StrV("picodata"), "Picodata (pgproto)").
				// doc: https://ydb.tech/docs/en/concepts/connect
				Opt(schemapb.StrV("ydb_grpc"), "YDB (grpc://)").
				Opt(schemapb.StrV("ydb_grpcs"), "YDB (grpcs://, TLS)").
				// doc: https://www.cockroachlabs.com/docs/stable/connection-parameters
				Opt(schemapb.StrV("cockroach"), "CockroachDB").
				Default(schemapb.StrV("pg")).Required(),

			schemapb.Str("dsn").Title("DSN").Group("Connection").
				Desc("Full connection string including credentials; stored encrypted and never rendered into logs or artifacts.").
				MinLen(3).MaxLen(4096).Secret().Required().
				Examples(
					schemapb.StrV("postgres://user:pass@db.internal:5432/stroppy?sslmode=require"),
					schemapb.StrV("grpcs://ydb.internal:2135/?database=/Root/stroppy"),
				),

			schemapb.Bool("tls_skip_verify").Title("Skip TLS verification").Group("Connection").
				Desc("Accept the server certificate without checking the chain or hostname; needed for self-signed stands, never for a shared endpoint.").
				Default(false),

			schemapb.Str("probe_query").Title("Probe query").Group("Health").
				Desc("Statement run once before the workload to prove the DSN works and the database answers.").
				MinLen(1).MaxLen(1024).Default("SELECT 1"),

			schemapb.Bool("truncate_before_run").Title("Clean before run").Group("Health").
				Desc("Let the workload drop and recreate its own tables. Off by default: this database is not ours to wipe.").
				Default(false),
		).
		Rules(
			schemapb.Rule(`root.protocol != "ydb_grpcs" || !root.tls_skip_verify`,
				"grpcs exists to verify the endpoint; skipping verification makes it grpc with extra cost").
				ID("grpcs-verify").Warn(),
		).
		MustBuild()
}
