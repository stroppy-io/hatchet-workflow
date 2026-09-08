// Conventions shared by the cfg.* schemas (the package doc lives in all.go).
//
// Two conventions are shared by every schema here and worth stating once.
//
//   - **Booleans are modeled as on/off Choice fields, not Bool.** The
//     Mustache render context is one level deep and exposes per-field
//     display forms only through the `fields` list, which a logic-less
//     template cannot address by name; `values.<name>` of a Bool renders
//     "true"/"false". PostgreSQL, PgBouncer and Patroni all accept (and
//     PostgreSQL's own docs use) the literal tokens on/off, so a two-option
//     Choice is both the faithful file syntax and the only form that renders
//     with `{{{values.x}}}`. Forms show it as a two-option select.
//
//   - **Lists and maps are rendered through Computed strings.** The render
//     context does not expand nested values, so any repeated block
//     (shared_preload_libraries, pg_hba records, etcd peers, the `custom`
//     escape hatch) is joined in CEL into a Computed string field, and the
//     template just prints it.
//
// Cluster-filled fields (peers, primary address, node name) live in group
// "Cluster", are Nullable with no default, and are filled by the server from
// the topology before Bake.

package cfg

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"
)

// customKeyPattern is the key syntax accepted by the `custom` escape hatch of
// the postgresql.conf family: a GUC name, optionally namespaced by an
// extension prefix.
const customKeyPattern = `^[a-z_][a-z0-9_.]*$`

// onOff builds a boolean-valued config field as an on/off choice (see the
// package doc for why Bool is not used).
func onOff(name schemapb.FieldName, def string) *schemapb.ChoiceB {
	return schemapb.Choice(name).
		Opt(schemapb.StrV("on"), "on").
		Opt(schemapb.StrV("off"), "off").
		Default(schemapb.StrV(def))
}

// trueFalse builds a boolean-valued YAML field as a true/false choice, for
// files (Patroni, etcd) whose syntax is YAML rather than key = value.
func trueFalse(name schemapb.FieldName, def string) *schemapb.ChoiceB {
	return schemapb.Choice(name).
		Opt(schemapb.StrV("true"), "true").
		Opt(schemapb.StrV("false"), "false").
		Default(schemapb.StrV(def))
}

// customField is the `custom` map every postgresql.conf-shaped schema carries
// for keys the schema does not model.
func customField() *schemapb.MapB {
	return schemapb.MapOf("custom", schemapb.Str("value")).
		Title("Extra settings").Group("Custom").
		Desc("Raw `key = value` settings appended verbatim at the end of the file; " +
			"keys must match " + customKeyPattern + ". Entry order is not preserved.")
}

// customRendered is the Computed counterpart of customField: the map joined
// into `key = value` lines.
func customRendered() *schemapb.ComputedB {
	return schemapb.Computed("custom_rendered",
		`("custom" in root) ? root.custom.map(k, k + " = " + string(root.custom[k])).join("\n") : ""`).
		Title("Rendered extra settings").Group("Custom").
		Desc("The `custom` map joined into config lines; the template prints it at the end of the file.").
		Result(schemapb.ResultString)
}

// customKeyRule enforces customKeyPattern on the keys of `custom`.
func customKeyRule() *schemapb.RuleB {
	return schemapb.Rule(
		`!("custom" in root) || root.custom.all(k, k.matches("`+customKeyPattern+`"))`,
		"custom keys must match "+customKeyPattern).ID("custom-keys")
}
