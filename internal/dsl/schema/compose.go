package schema

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ProviderSchemas is one provider's derived schema fragments, as produced by
// DeriveParamsSchema: Params is the provider's params object schema (for
// core.schema.json's $defs.providerParams, referenced from
// provider.params:), Ext is the schema of its "stroppy_machine_ext"
// variable (for $defs.machineExt, referenced from every machine-group's
// additionalProperties).
type ProviderSchemas struct {
	Params map[string]any
	Ext    map[string]any
}

// composedSchemaURL is the $id under which Compose (via CompileComposed)
// registers its generated document with a fresh jsonschema.Compiler; like
// coreSchemaURL it never needs to resolve over the network -- it is only a
// resource key.
const composedSchemaURL = "https://stroppy.io/dsl/composed.schema.json"

// includeInputsDefPrefix names the $defs entries Compose adds for
// `fragments`: for a fragments key "foo", Compose stores its inputs schema
// verbatim at $defs["includeInputs_foo"] (directly $ref-able as
// "#/$defs/includeInputs_foo"), following the same convention every other
// core.schema.json $defs entry already uses -- a bare schema object, not a
// name->schema lookup table nested one level deeper. Compose itself does
// not add a $ref to these from anywhere in the document (nothing in
// core.schema.json's cluster/workflow/component shapes points at an
// "include:" mechanism yet); they exist so a consumer building an
// IDE-facing schema, or a future include-resolution pass (Task 8/18), can
// look one up by name and $ref it explicitly.
const includeInputsDefPrefix = "includeInputs_"

// Compose builds a document-specific JSON Schema by taking the embedded
// core.schema.json and replacing its two provider-extension points --
// $defs.providerParams and $defs.machineExt, both permissive objects in the
// embedded core -- with schemas derived from providers, then adding one
// $defs.includeInputs_<name> entry per fragments key (see
// includeInputsDefPrefix).
//
// providers is keyed by provider name (e.g. "yandex") purely for caller
// bookkeeping; Compose itself only looks at the values. A document only
// ever activates one provider via `provider.use:`, but core.schema.json's
// $defs.providerParams/$defs.machineExt are each a single schema slot
// shared by whichever provider-specific keys appear in the document (e.g.
// machine-group additionalProperties has no way to know a given key came
// from "yandex" specifically), so Compose does not know in advance which
// provider(s) a given document will use:
//   - zero entries in providers: the corresponding $defs entry is left as
//     core.schema.json's original permissive placeholder.
//   - exactly one entry: the corresponding $defs entry becomes that
//     provider's schema verbatim.
//   - two or more entries: the corresponding $defs entry becomes
//     {"anyOf": [schema, schema, ...]} (any one provider's shape is
//     accepted; branch order is unspecified since map iteration order is
//     random, but that doesn't affect validation outcomes).
//
// Compose returns the schema compiled to the cluster document shape
// (core.schema.json's $defs.cluster is the only per-kind entry point that
// references providerParams/machineExt today) -- pass it directly as
// Validate's `composed` argument together with schema.Cluster. The []byte
// return is the full composed document (every $defs entry, not just
// cluster's), suitable for handing to an IDE JSON-Schema endpoint or for
// dispatching to a different kind via CompileComposed.
func Compose(providers map[string]ProviderSchemas, fragments map[string]map[string]any) (*jsonschema.Schema, []byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(coreSchemaJSON, &doc); err != nil {
		return nil, nil, fmt.Errorf("dsl/schema: parse embedded core.schema.json: %w", err)
	}
	doc["$id"] = composedSchemaURL

	defs, ok := doc["$defs"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("dsl/schema: embedded core.schema.json has no $defs object")
	}

	if params := mergeProviderSchemas(providers, func(p ProviderSchemas) map[string]any { return p.Params }); params != nil {
		defs["providerParams"] = params
	}
	if ext := mergeProviderSchemas(providers, func(p ProviderSchemas) map[string]any { return p.Ext }); ext != nil {
		defs["machineExt"] = ext
	}
	for name, fragment := range fragments {
		defs[includeInputsDefPrefix+name] = fragment
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, fmt.Errorf("dsl/schema: marshal composed schema: %w", err)
	}

	compiled, err := CompileComposed(raw, Cluster)
	if err != nil {
		return nil, nil, err
	}
	return compiled, raw, nil
}

// CompileComposed compiles a raw composed schema document (Compose's
// []byte return, or an equivalent hand-built document) and returns the
// compiled entry point for kind, exactly like the embedded core schema's
// compileCore/kindSchema do for MustCore(). It is exported (unlike
// compileCore) so a caller holding Compose's raw []byte can dispatch the
// SAME composed document to whichever of the three per-kind entry points
// (cluster/workflow/component) it actually needs -- Compose's own
// *jsonschema.Schema return only ever gives you the cluster one, since
// that's the sole shape providerParams/machineExt participate in today.
func CompileComposed(raw []byte, kind Kind) (*jsonschema.Schema, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("dsl/schema: parse composed schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(composedSchemaURL, doc); err != nil {
		return nil, fmt.Errorf("dsl/schema: add composed schema resource: %w", err)
	}

	defName, err := kindDefName(kind)
	if err != nil {
		return nil, err
	}
	compiled, err := compiler.Compile(composedSchemaURL + "#/$defs/" + defName)
	if err != nil {
		return nil, fmt.Errorf("dsl/schema: compile composed $defs/%s: %w", defName, err)
	}
	return compiled, nil
}

// kindDefName maps a Kind to its core.schema.json $defs entry name.
func kindDefName(kind Kind) (string, error) {
	switch kind {
	case Cluster:
		return "cluster", nil
	case Workflow:
		return "workflow", nil
	case Component:
		return "component", nil
	default:
		return "", fmt.Errorf("dsl/schema: unknown Kind %d", int(kind))
	}
}

// mergeProviderSchemas extracts one field (Params or Ext, via get) from
// every entry of providers and combines them per Compose's doc comment:
// nil for zero entries, the schema itself for exactly one, an "anyOf" of
// all of them for two or more. Entries where get returns nil (a provider
// with no Ext, say) are skipped.
func mergeProviderSchemas(providers map[string]ProviderSchemas, get func(ProviderSchemas) map[string]any) map[string]any {
	var schemas []map[string]any
	for _, p := range providers {
		if s := get(p); s != nil {
			schemas = append(schemas, s)
		}
	}
	switch len(schemas) {
	case 0:
		return nil
	case 1:
		return schemas[0]
	default:
		anyOf := make([]any, len(schemas))
		for i, s := range schemas {
			anyOf[i] = s
		}
		return map[string]any{"anyOf": anyOf}
	}
}
