package schema

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"

	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

// DeriveProviderParamsSchemapb reads a provider Terraform module's
// variables.tf (via terraform-config-inspect) and derives two schemapb
// schemas from it, mirroring DeriveParamsSchema's JSON-Schema derivation
// but targeting the richer schemapb model (typed fields, defaults, secret
// flags, and validation-block-derived rules):
//   - params: an object schema with one field per declared variable
//     (excluding machineExtVarName).
//   - ext: the schema of the machineExtVarName variable's inner fields
//     (expected to be an object() type), nil if the module declares no
//     such variable.
//
// Best-effort: an unparseable variable degrades to a permissive string
// field, and an untranslatable validation condition is reported as a
// warning diagnostic -- neither fails the whole module. This is one of the
// few functions in internal/dsl that performs filesystem I/O (via
// tfconfig.LoadModule and reading the variables.tf source for validation
// blocks); every other dsl package/function is pure.
func DeriveProviderParamsSchemapb(moduleDir, providerName string) (params, ext *schemapb.Schema, diags diag.List) {
	mod, loadDiags := tfconfig.LoadModule(moduleDir)
	if err := loadDiags.Err(); err != nil {
		diags.Errorf(moduleDir, diag.Pos{}, "load terraform module: %v", err)
		return nil, nil, diags
	}

	names := make([]string, 0, len(mod.Variables))
	for name := range mod.Variables {
		names = append(names, name)
	}
	sort.Strings(names)

	var paramFields []schemapb.FieldDef
	var extFields []schemapb.FieldDef
	var rules []schemapb.RuleDef
	fileContentCache := make(map[string]string)
	for _, name := range names {
		v := mod.Variables[name]
		f := fieldForTFType(name, v.Type, v.Description, v.Default, v.Sensitive)

		if name == machineExtVarName {
			extFields = objectFieldsOf(f)
			continue
		}
		paramFields = append(paramFields, f)

		for _, val := range validationBlocksForVariable(v, fileContentCache) {
			celExpr, ok := translateValidationCondition(val.Condition)
			if !ok {
				diags.Warnf(v.Pos.Filename, diag.Pos{Line: v.Pos.Line},
					"variable %q: cannot translate validation condition %q to an expression rule; skipping", name, val.Condition)
				continue
			}
			rules = append(rules, schemapb.Rule(celExpr, val.ErrorMessage))
		}
	}

	builder := schemapb.NewSchema("stroppy.provider."+providerName, "params", "1").Fields(paramFields...)
	if len(rules) > 0 {
		builder = builder.Rules(rules...)
	}
	built, err := builder.Build()
	if err != nil {
		diags.Errorf(moduleDir, diag.Pos{}, "build params schema: %v", err)
		return nil, nil, diags
	}

	if extFields != nil {
		builtExt, err := schemapb.NewSchema("stroppy.provider."+providerName, "machine_ext", "1").
			Fields(extFields...).Build()
		if err != nil {
			diags.Errorf(moduleDir, diag.Pos{}, "build machine_ext schema: %v", err)
		} else {
			ext = builtExt
		}
	}

	return built, ext, diags
}

// fieldForTFType maps a Terraform variable's type-constraint expression and
// metadata (description, default, sensitive) to a schemapb field builder.
// It is the schemapb counterpart of tfvars.go's parseType, reusing its
// balanced-expression scanners (parseCall/parseObjectBody/splitTopLevel)
// instead of re-implementing the type-constraint grammar.
//
// NOTE on real builder API vs. the plan's sketch: schemapb's fluent builders
// are named Str/Double/Bool/List/Object (not String/Int64), and the shared
// field modifier is `.Desc(...)` (not `.Descr(...)`); this function was
// adapted to match schemapb v1.4.4's actual new.go API.
func fieldForTFType(name, tfType, description string, def any, sensitive bool) schemapb.FieldDef {
	s := strings.TrimSpace(tfType)
	switch s {
	case "string":
		b := schemapb.Str(name)
		applyStrCommon(b, description, sensitive, def)
		return b
	case "number":
		b := schemapb.Double(name)
		applyDoubleCommon(b, description, sensitive, def)
		return b
	case "bool":
		b := schemapb.Bool(name)
		applyBoolCommon(b, description, sensitive, def)
		return b
	case "any", "":
		// Permissive fallback: schemapb has no "any" kind, so an untyped TF
		// variable becomes an unconstrained string field.
		b := schemapb.Str(name)
		applyStrCommon(b, description, sensitive, nil)
		return b
	}

	keyword, inner, ok := parseCall(s)
	if !ok {
		b := schemapb.Str(name)
		applyStrCommon(b, description, sensitive, nil)
		return b
	}

	switch keyword {
	case "list", "set":
		item := fieldForTFType("item", inner, "", nil, false)
		b := schemapb.List(name, item)
		if description != "" {
			b.Desc(description)
		}
		if sensitive {
			b.Secret()
		}
		return b
	case "map":
		// v1 fallback: schemapb has no map(T)-with-typed-values kind, so a TF
		// map(T) becomes a permissive object with no fixed properties.
		//
		// Deliberately NOT .Strict(): map(T) (and map(object({...})) in
		// particular, e.g. yandex's subnets/vms variables) has free,
		// user-chosen keys by design (subnet names, VM names) -- Strict()
		// would reject every legitimate config. This is also why this
		// fallback cannot validate the VALUE side either: schemapb's Object
		// kind only knows how to declare a fixed set of named fields, so
		// there is nowhere to attach `inner`'s value type/shape at all. That
		// is a genuine schemapb gap (no Map kind), not something fixable
		// here -- see SP-I1's report (.superpowers/sdd/spd-i1-fix-report.md)
		// before attempting to "fix" this by hand.
		b := schemapb.Object(name)
		if description != "" {
			b.Desc(description)
		}
		if sensitive {
			b.Secret()
		}
		return b
	case "object":
		body := strings.TrimSpace(inner)
		body = strings.TrimPrefix(body, "{")
		body = strings.TrimSuffix(body, "}")
		// A terraform object({...}) type constraint is a FIXED attribute set
		// (unlike map(object({...})) above, whose keys are arbitrary and
		// user-chosen by design) -- so unknown keys under it must be
		// rejected. .Strict() here closes the "config-injection" hole
		// commit 056becbb only closed at the top level (see SP-I1): since
		// fieldForTFType calls itself recursively for nested object()
		// attributes (via objectFieldsFromBody -> attrFieldFromExpr), every
		// nested plain object() gets its own Strict() the same way, and
		// schemapb's validator checks each nested Schema's own Strict flag
		// independently (schemapb/validate.go's checkObject ->
		// validateFields), so this is strict at every depth without any
		// extra recursion here.
		b := schemapb.Object(name, objectFieldsFromBody(body)...).Strict()
		if description != "" {
			b.Desc(description)
		}
		if sensitive {
			b.Secret()
		}
		return b
	default:
		b := schemapb.Str(name)
		applyStrCommon(b, description, sensitive, nil)
		return b
	}
}

func applyStrCommon(b *schemapb.StrB, description string, sensitive bool, def any) {
	if description != "" {
		b.Desc(description)
	}
	if sensitive {
		b.Secret()
	}
	if sv, ok := def.(string); ok {
		b.Default(sv)
	}
}

func applyDoubleCommon(b *schemapb.DoubleB, description string, sensitive bool, def any) {
	if description != "" {
		b.Desc(description)
	}
	if sensitive {
		b.Secret()
	}
	if dv, ok := toFloat64(def); ok {
		b.Default(dv)
	}
}

func applyBoolCommon(b *schemapb.BoolB, description string, sensitive bool, def any) {
	if description != "" {
		b.Desc(description)
	}
	if sensitive {
		b.Secret()
	}
	if bv, ok := def.(bool); ok {
		b.Default(bv)
	}
}

// toFloat64 converts a numeric default value decoded by tfconfig (JSON
// unmarshaling yields float64 for all JSON numbers) to a float64. Any other
// shape (type mismatch) reports ok=false so the caller skips the default
// rather than guessing.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// objectFieldsFromBody parses the inside of an object({ ... }) type
// constraint (the part between the braces) into schemapb field builders.
// It mirrors tfvars.go's parseObjectBody traversal (reusing its
// splitTopLevel/splitFirstTopLevelEquals scanners) but emits schemapb
// FieldDefs instead of JSON Schema fragments; optional(...) defaults are
// applied per-kind the same way fieldForTFType applies variable-level
// defaults. Like parseObjectBody, a bare (non-optional(...)) attribute is
// required -- that is marked on the built field via Schema_Filed.Required
// (schemapb's fieldBase.Required() setter, applied here through the
// Done() escape hatch since attrFieldFromExpr returns the FieldDef
// interface rather than a concrete kind-specific builder).
func objectFieldsFromBody(body string) []schemapb.FieldDef {
	var fields []schemapb.FieldDef
	for _, entry := range splitTopLevel(body, isCommaOrNewline) {
		key, valueExpr, ok := splitFirstTopLevelEquals(entry)
		if !ok {
			continue // malformed attribute definition; skip (permissive fallback)
		}
		key = strings.Trim(key, `"`)
		field, optional := attrFieldFromExpr(key, valueExpr)
		if !optional {
			field.Done().Required = true
		}
		fields = append(fields, field)
	}
	return fields
}

// attrFieldFromExpr builds a schemapb field for one object() attribute's
// value expression: either a bare type (required attribute, optional=false)
// or optional(type[, default]) (optional attribute, optional=true, with the
// default applied per-kind if given). Mirrors tfvars.go's parseAttrExpr
// bare-vs-optional return shape so callers can track required-ness the same
// way parseObjectBody does for the JSON Schema path.
func attrFieldFromExpr(name, valueExpr string) (field schemapb.FieldDef, optional bool) {
	keyword, inner, ok := parseCall(valueExpr)
	if !ok || keyword != "optional" {
		return fieldForTFType(name, valueExpr, "", nil, false), false
	}

	parts := splitTopLevel(inner, isComma)
	if len(parts) == 0 {
		return schemapb.Str(name), true
	}
	var def any
	if len(parts) > 1 {
		def = parseLiteral(strings.Join(parts[1:], ","))
	}
	return fieldForTFType(name, parts[0], "", def, false), true
}

// objectFieldsOf returns f's inner fields if f is an object-kind field
// (used to flatten the stroppy_machine_ext variable's object() type into
// the standalone ext schema's top-level fields); otherwise it wraps f
// itself as a single-element slice (best-effort: a non-object
// stroppy_machine_ext variable still produces a usable, if minimal, ext
// schema instead of failing).
func objectFieldsOf(f schemapb.FieldDef) []schemapb.FieldDef {
	done := f.Done()
	if obj := done.GetObject(); obj != nil && obj.GetSchema() != nil {
		inner := obj.GetSchema().GetFields()
		out := make([]schemapb.FieldDef, len(inner))
		for i, ff := range inner {
			out[i] = ff
		}
		return out
	}
	return []schemapb.FieldDef{done}
}

// --- validation {} block -> schemapb rule -----------------------------------
//
// terraform-config-inspect's tfconfig.Variable does not expose validation
// blocks (see tfconfig/variable.go), so this reads the raw source file the
// variable was declared in (v.Pos.Filename, populated by tfconfig itself)
// and scans it with the same balanced-brace style approach as tfvars.go's
// scanners for the variable's `validation { condition = ...; error_message
// = ... }` block(s).

// tfValidation is one parsed `validation { condition = ...; error_message =
// ... }` block.
type tfValidation struct {
	Condition    string
	ErrorMessage string
}

// validationBlocksForVariable returns every validation block declared
// inside v's `variable "name" { ... }` block. Returns nil (never an error)
// if the source file can't be read or no validation block is found --
// validation-rule derivation is a best-effort enrichment, not something
// that should fail schema derivation.
//
// fileContentCache is scoped to a single DeriveProviderParamsSchemapb call
// (created fresh by the caller and threaded through here) so that repeated
// reads of the same source file within that call are deduped without
// sharing mutable state across concurrent calls -- a package-level cache
// written here without synchronization would be a concurrent-map-write
// crash risk if this function is ever called from multiple goroutines.
func validationBlocksForVariable(v *tfconfig.Variable, fileContentCache map[string]string) []tfValidation {
	if v.Pos.Filename == "" {
		return nil
	}
	content, ok := fileContentCache[v.Pos.Filename]
	if !ok {
		raw, err := os.ReadFile(v.Pos.Filename)
		if err != nil {
			return nil
		}
		content = string(raw)
		fileContentCache[v.Pos.Filename] = content
	}

	body, ok := findVariableBlockBody(content, v.Name)
	if !ok {
		return nil
	}
	return extractValidationBlocks(body)
}

// findVariableBlockBody locates `variable "name" { ... }` in src and
// returns the text strictly between its matching outer braces.
func findVariableBlockBody(src, name string) (string, bool) {
	needle := fmt.Sprintf("variable %q", name)
	_, rest, found := strings.Cut(src, needle)
	if !found {
		return "", false
	}
	openIdx := strings.IndexByte(rest, '{')
	if openIdx < 0 {
		return "", false
	}
	return extractBalancedBrace(rest, openIdx)
}

// extractValidationBlocks finds every top-level `validation { ... }` block
// inside a variable block's body and parses each into a tfValidation.
func extractValidationBlocks(body string) []tfValidation {
	var out []tfValidation
	rest := body
	for {
		idx := strings.Index(rest, "validation")
		if idx < 0 {
			return out
		}
		after := rest[idx+len("validation"):]
		openIdx := strings.IndexByte(after, '{')
		if openIdx < 0 {
			return out
		}
		if strings.TrimSpace(after[:openIdx]) != "" {
			// Not actually a `validation {` block (e.g. part of a longer
			// identifier or a comment); keep scanning past this match.
			rest = after
			continue
		}
		inner, ok := extractBalancedBrace(after, openIdx)
		if !ok {
			return out
		}
		if v, ok := parseValidationBlockBody(inner); ok {
			out = append(out, v)
		}
		rest = after[openIdx+1+len(inner):]
	}
}

// parseValidationBlockBody parses the inside of one `validation { ... }`
// block into its condition expression and error_message string.
func parseValidationBlockBody(inner string) (tfValidation, bool) {
	var v tfValidation
	for _, line := range splitTopLevel(inner, isNewline) {
		key, val, ok := splitFirstTopLevelEquals(line)
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "condition":
			v.Condition = strings.TrimSpace(val)
		case "error_message":
			if s, ok := parseLiteral(val).(string); ok {
				v.ErrorMessage = s
			} else {
				v.ErrorMessage = strings.Trim(strings.TrimSpace(val), `"`)
			}
		}
	}
	if v.Condition == "" || v.ErrorMessage == "" {
		return tfValidation{}, false
	}
	return v, true
}

func isNewline(b byte) bool { return b == '\n' }

// extractBalancedBrace returns the text strictly between s[openIdx] (which
// must be '{') and its matching '}', tracking nested braces and skipping
// over quoted strings. It is the brace-matching analogue of tfvars.go's
// extractBalancedParen, needed here to scan raw HCL blocks (variable { },
// validation { }) rather than type-constraint call expressions.
func extractBalancedBrace(s string, openIdx int) (string, bool) {
	depth := 0
	inQuote := false
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				inQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[openIdx+1 : i], true
			}
		}
	}
	return "", false
}

// varRefRe matches a `var.<identifier>` reference inside a validation
// condition expression.
var varRefRe = regexp.MustCompile(`var\.([A-Za-z_][A-Za-z0-9_]*)`)

// safeConditionRe allow-lists the character set translateValidationCondition
// accepts: identifiers, numbers, `.`, comparison/logical/arithmetic
// operators, parens, and whitespace. Anything else (string literals,
// function calls like `contains(...)`, etc.) is refused rather than
// mistranslated -- see translateValidationCondition's doc comment.
var safeConditionRe = regexp.MustCompile(`^[A-Za-z0-9_.()!<>=&|+\-*/\s]+$`)

// translateValidationCondition translates a simple Terraform validation
// condition expression (referencing the variable being validated, and
// possibly other variables, via `var.NAME`) into a schemapb rule expression
// (schemapb rules are evaluated with `root` bound to the values map, so
// `var.NAME` becomes `root.NAME` -- see schemapb/new.go's ExampleRule).
//
// Only simple expressions survive: comparisons/boolean combinations over
// var.* references, numeric literals, and arithmetic (e.g.
// "var.replicas >= 1", "var.min <= var.max && var.max <= 100"). Anything
// containing a construct outside that allow-list (string literals,
// function calls, indexing, ...) is refused (ok=false) so the caller can
// degrade to a warning instead of emitting a rule expression schemapb's
// validator would reject or -- worse -- silently misevaluate.
func translateValidationCondition(cond string) (string, bool) {
	cond = strings.TrimSpace(cond)
	if cond == "" || !safeConditionRe.MatchString(cond) {
		return "", false
	}
	if !strings.Contains(cond, "var.") {
		return "", false
	}
	return varRefRe.ReplaceAllString(cond, "root.$1"), true
}
