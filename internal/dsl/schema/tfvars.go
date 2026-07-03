package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

// machineExtVarName is the well-known variables.tf variable name a provider
// module uses to declare the shape of the per-machine extension block it
// accepts (e.g. cluster.yaml's `machines.db.yandex: {...}`, validated
// against core.schema.json's $defs.machineExt once Compose wires it in).
// DeriveParamsSchema splits it out of the ordinary params properties into
// its own return value.
const machineExtVarName = "stroppy_machine_ext"

// DeriveParamsSchema reads a provider Terraform module's variables.tf (via
// terraform-config-inspect) and derives two JSON Schema fragments from it:
//   - params: an object schema (additionalProperties: false) with one
//     property per declared variable (excluding machineExtVarName), for
//     core.schema.json's $defs.providerParams (provider.params:).
//   - ext: the schema of the machineExtVarName variable alone (expected to
//     be an object() type), for $defs.machineExt (machine-group additional
//     properties); nil if the module declares no such variable.
//
// This is the only function in internal/dsl that performs filesystem I/O
// (via tfconfig.LoadModule) -- every other dsl package/function is pure.
func DeriveParamsSchema(moduleDir string) (params, ext map[string]any, err error) {
	mod, diags := tfconfig.LoadModule(moduleDir)
	if diagErr := diags.Err(); diagErr != nil {
		return nil, nil, fmt.Errorf("dsl/schema: load terraform module %q: %w", moduleDir, diagErr)
	}

	properties := make(map[string]any, len(mod.Variables))
	for name, v := range mod.Variables {
		varSchema := parseType(v.Type)
		if v.Description != "" {
			varSchema["description"] = v.Description
		}
		if v.Default != nil {
			varSchema["default"] = v.Default
		}
		if name == machineExtVarName {
			ext = varSchema
			continue
		}
		properties[name] = varSchema
	}

	params = map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	return params, ext, nil
}

// parseType converts a Terraform variable type-constraint expression (as
// terraform-config-inspect returns it: raw source text, e.g. "string",
// "list(string)", or "object({...})" with the original whitespace) into a
// JSON Schema fragment. It handles primitives (string/number/bool/any),
// list(T)/set(T) (-> array), map(T) (-> object with additionalProperties:
// schema of T), and object({k = T, k2 = optional(T2, default), ...})
// (-> object with properties/required/additionalProperties: false).
//
// It does not implement the full HCL type-constraint grammar (e.g. no
// tuple(...) support). Any type expression it does not recognize -- an
// unparseable string, or a keyword it doesn't handle -- silently falls back
// to a permissive `{}` schema (accepts anything) instead of returning an
// error: DeriveParamsSchema's contract is "best effort schema, never fail
// the whole module over one unusual variable."
func parseType(raw string) map[string]any {
	s := strings.TrimSpace(raw)
	switch s {
	case "string":
		return map[string]any{"type": "string"}
	case "number":
		return map[string]any{"type": "number"}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "any", "":
		return map[string]any{}
	}

	keyword, inner, ok := parseCall(s)
	if !ok {
		return map[string]any{}
	}
	switch keyword {
	case "list", "set":
		return map[string]any{"type": "array", "items": parseType(inner)}
	case "map":
		return map[string]any{"type": "object", "additionalProperties": parseType(inner)}
	case "object":
		body := strings.TrimSpace(inner)
		body = strings.TrimPrefix(body, "{")
		body = strings.TrimSuffix(body, "}")
		props, required := parseObjectBody(body)
		out := map[string]any{
			"type":                 "object",
			"properties":           props,
			"additionalProperties": false,
		}
		if len(required) > 0 {
			sort.Strings(required)
			out["required"] = required
		}
		return out
	default:
		return map[string]any{}
	}
}

// parseObjectBody parses the inside of an object({ ... }) type constraint
// (the part between the braces): a sequence of `name = type` or
// `name = optional(type, default)` attribute definitions, separated by
// commas and/or newlines (HCL's object type constructor allows either). It
// returns the derived per-attribute schemas and the subset of attribute
// names that are NOT wrapped in optional(...) (Terraform: an object()
// attribute is required unless declared via optional()).
func parseObjectBody(body string) (props map[string]any, required []string) {
	props = map[string]any{}
	for _, entry := range splitTopLevel(body, isCommaOrNewline) {
		key, valueExpr, ok := splitFirstTopLevelEquals(entry)
		if !ok {
			continue // malformed attribute definition; skip (permissive fallback)
		}
		key = strings.Trim(key, `"`)
		attrSchema, optional := parseAttrExpr(valueExpr)
		props[key] = attrSchema
		if !optional {
			required = append(required, key)
		}
	}
	return props, required
}

// parseAttrExpr parses one object() attribute's value expression: either a
// bare type (required attribute) or optional(type[, default]) (optional
// attribute, with a captured JSON Schema "default" annotation if a default
// value was given).
func parseAttrExpr(valueExpr string) (attrSchema map[string]any, optional bool) {
	keyword, inner, ok := parseCall(valueExpr)
	if !ok || keyword != "optional" {
		return parseType(valueExpr), false
	}

	parts := splitTopLevel(inner, isComma)
	if len(parts) == 0 {
		return map[string]any{}, true
	}
	sch := parseType(parts[0])
	if len(parts) > 1 {
		sch["default"] = parseLiteral(strings.Join(parts[1:], ","))
	}
	return sch, true
}

// parseLiteral parses a single HCL literal expression (a default value
// inside optional(type, default)) into a Go value suitable for a JSON
// Schema "default" annotation: quoted strings (with escapes), true/false,
// null, integers, floats, and -- as a last resort -- anything valid JSON
// (covers simple list/map literals like [] or {}). Anything else is kept as
// its raw source text.
func parseLiteral(s string) any {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if unquoted, err := strconv.Unquote(s); err == nil {
			return unquoted
		}
		return strings.Trim(s, `"`)
	}
	switch s {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// --- small balanced-expression scanning helpers ---
//
// These treat the Terraform type-constraint grammar as a flat token stream:
// they don't build an AST, they just find matching '(' ... ')' pairs and
// split on top-level separators, skipping over anything inside a quoted
// string (so a default like "a,b" or "a(b)" doesn't confuse the scan). This
// is deliberately far short of a full HCL parser -- see parseType's doc
// comment on unparseable input falling back to `{}`.

func isComma(b byte) bool          { return b == ',' }
func isCommaOrNewline(b byte) bool { return b == ',' || b == '\n' }

// parseCall recognizes a `keyword(...)` expression and returns the keyword
// and the raw text between the matching outer parens. ok is false if s has
// no top-level '(' at all (e.g. a bare "string").
func parseCall(s string) (keyword, argsInner string, ok bool) {
	s = strings.TrimSpace(s)
	idx := strings.IndexByte(s, '(')
	if idx < 0 {
		return "", "", false
	}
	keyword = strings.TrimSpace(s[:idx])
	if keyword == "" {
		return "", "", false
	}
	inner, ok := extractBalancedParen(s, idx)
	return keyword, inner, ok
}

// extractBalancedParen returns the text strictly between s[openIdx] (which
// must be '(') and its matching ')', tracking nested parens and skipping
// over quoted strings.
func extractBalancedParen(s string, openIdx int) (string, bool) {
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
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[openIdx+1 : i], true
			}
		}
	}
	return "", false
}

// splitTopLevel splits s on every byte for which isSep returns true at
// nesting depth 0 (depth counts unmatched '(', '{', '[' regardless of which
// close matches, since here we only need "am I nested", not exact
// pairing), skipping separators inside quoted strings. Empty tokens
// (consecutive separators, leading/trailing whitespace-only) are dropped.
func splitTopLevel(s string, isSep func(byte) bool) []string {
	var out []string
	depth := 0
	inQuote := false
	start := 0
	flush := func(end int) {
		tok := strings.TrimSpace(s[start:end])
		if tok != "" {
			out = append(out, tok)
		}
		start = end + 1
	}
	for i := 0; i < len(s); i++ {
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
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		default:
			if depth == 0 && isSep(c) {
				flush(i)
			}
		}
	}
	flush(len(s))
	return out
}

// splitFirstTopLevelEquals splits entry ("name = type-expr") at its first
// top-level '=' (skipping '=' inside nested parens/braces/brackets or
// quoted strings, though none of those should appear before a valid
// attribute's '=' in practice).
func splitFirstTopLevelEquals(entry string) (key, value string, ok bool) {
	depth := 0
	inQuote := false
	for i := 0; i < len(entry); i++ {
		c := entry[i]
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
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case '=':
			if depth == 0 {
				return strings.TrimSpace(entry[:i]), strings.TrimSpace(entry[i+1:]), true
			}
		}
	}
	return "", "", false
}
