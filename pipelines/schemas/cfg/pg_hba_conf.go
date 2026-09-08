package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// PgHbaConf1 is cfg.pg_hba.conf@1 — the client authentication file.
//
// Record grammar and every field's allowed values verified against
// https://www.postgresql.org/docs/18/auth-pg-hba-conf.html (the grammar is
// unchanged across 15..18; `oauth` is the only method added since 15 and is
// listed as such in the method Desc).
//
// The file is a list, and the render context does not expand nested values, so
// the records are joined into the Computed field `rules_rendered` in CEL and
// the template just prints that block.
func PgHbaConf1() *schemapb.Schema {
	return schemapb.NewSchema(ids.Cfg("pg_hba.conf", 1)).
		Descr("pg_hba.conf: ordered client authentication records.").
		Strict().Coerce().
		Fields(
			schemapb.List("rules", schemapb.Object("rule",
				// doc: auth-pg-hba-conf.html — connection type
				schemapb.Choice("type").Title("Type").Group("Record").
					Desc("Connection type the record matches.").
					Opt(schemapb.StrV("local"), "local (unix socket)").
					Opt(schemapb.StrV("host"), "host (TCP, any encryption)").
					Opt(schemapb.StrV("hostssl"), "hostssl (TCP, SSL only)").
					Opt(schemapb.StrV("hostnossl"), "hostnossl (TCP, no SSL)").
					Default(schemapb.StrV("host")).Required(),
				// doc: auth-pg-hba-conf.html — DATABASE field
				schemapb.Str("database").Title("Database").Group("Record").
					Desc("Database the record matches: a name, a comma-separated list, or one of all / sameuser / samerole / replication.").
					MinLen(1).MaxLen(512).Default("all").Required(),
				// doc: auth-pg-hba-conf.html — USER field
				schemapb.Str("user").Title("User").Group("Record").
					Desc("Role the record matches: a name, a comma-separated list, +group, or all.").
					MinLen(1).MaxLen(512).Default("all").Required(),
				// doc: auth-pg-hba-conf.html — ADDRESS field
				schemapb.Str("address").Title("Address").Group("Record").
					Desc("Client address for host records: CIDR (10.0.0.0/8, ::1/128), a hostname, or all / samehost / samenet. Must be empty for `local` records.").
					MaxLen(256).Nullable(),
				// doc: auth-pg-hba-conf.html — METHOD field
				schemapb.Choice("method").Title("Method").Group("Record").
					Desc("Authentication method. `oauth` exists only from PostgreSQL 18.").
					Opt(schemapb.StrV("trust"), "trust").
					Opt(schemapb.StrV("reject"), "reject").
					Opt(schemapb.StrV("scram-sha-256"), "scram-sha-256").
					Opt(schemapb.StrV("md5"), "md5 (deprecated)").
					Opt(schemapb.StrV("password"), "password (cleartext)").
					Opt(schemapb.StrV("peer"), "peer (local only)").
					Opt(schemapb.StrV("ident"), "ident").
					Opt(schemapb.StrV("cert"), "cert").
					Opt(schemapb.StrV("gss"), "gss").
					Opt(schemapb.StrV("ldap"), "ldap").
					Opt(schemapb.StrV("radius"), "radius").
					Opt(schemapb.StrV("pam"), "pam").
					Opt(schemapb.StrV("oauth"), "oauth (18+)").
					Default(schemapb.StrV("scram-sha-256")).Required(),
				// doc: auth-pg-hba-conf.html — auth-options
				schemapb.Str("options").Title("Options").Group("Record").
					Desc("Method options appended verbatim, e.g. `map=stroppy` or `clientcert=verify-full`.").
					MaxLen(512).Nullable(),
			).Strict().Rule(
				schemapb.Rule(`this.type == "local" ? !("address" in this) || this.address == "" : ("address" in this) && this.address != ""`,
					"address is required for host records and forbidden for local records").ID("address-per-type"),
			)).
				Title("Records").Group("Records").
				Desc("Authentication records in file order: PostgreSQL uses the first record that matches.").
				MaxItems(256).Nullable(),

			schemapb.Computed("rules_rendered",
				`("rules" in root) ? root.rules.map(r,`+
					` r.type + "\t" + r.database + "\t" + r.user`+
					` + (r.type == "local" ? "" : "\t" + r.address)`+
					` + "\t" + r.method`+
					` + (("options" in r) && r.options != "" ? "\t" + r.options : "")`+
					`).join("\n") : ""`).
				Title("Rendered records").Group("Records").
				Desc("The `rules` list joined into pg_hba.conf lines, tab-separated, in list order.").
				Result(schemapb.ResultString),

			schemapb.Choice("include_cluster_defaults").Title("Cluster defaults").Group("Cluster").
				Desc("Prepend the records the server derives from the topology (local superuser peer access, replication between the cluster peers, the workload role from the load-generator subnet). Filled by the server; `no` means `rules` is the whole file.").
				Opt(schemapb.StrV("yes"), "yes").Opt(schemapb.StrV("no"), "no").
				Default(schemapb.StrV("yes")),

			schemapb.Computed("cluster_defaults_rendered",
				`root.include_cluster_defaults == "yes" ? "local\tall\tpostgres\t\tpeer" : ""`).
				Title("Rendered cluster defaults").Group("Cluster").
				Desc("The unconditional record every stroppy node needs: local superuser access for the agent. The remaining topology records are appended by the server as `rules`.").
				Result(schemapb.ResultString),
		).
		Rules(
			schemapb.Rule(`("rules" in root) || root.include_cluster_defaults == "yes"`,
				"an empty pg_hba.conf would lock everyone out: set rules or keep the cluster defaults").
				ID("not-empty"),
		).
		Template("conf", `# pg_hba.conf — generated by stroppy.
# TYPE	DATABASE	USER	ADDRESS	METHOD	[OPTIONS]
{{#values.cluster_defaults_rendered}}{{{values.cluster_defaults_rendered}}}
{{/values.cluster_defaults_rendered}}{{#values.rules_rendered}}{{{values.rules_rendered}}}
{{/values.rules_rendered}}`).
		MustBuild()
}
