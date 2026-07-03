// Package schema validates stroppy YAML-DSL documents (cluster.yaml,
// workflow.yaml, component.yaml) against a JSON Schema (draft 2020-12)
// before the internal/dsl/ast package decodes them. The two layers overlap
// deliberately: schema validation gives IDE-shareable, editor-friendly
// feedback (it can be handed to any JSON-Schema-aware tool), while ast
// decoding additionally captures exact source positions and
// provider-specific extension blocks that a static schema cannot express.
// Schema validation does not replace ast's checks.
package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	_ "embed"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/stroppy-io/stroppy-cloud/internal/dsl/diag"
)

//go:embed core.schema.json
var coreSchemaJSON []byte

// coreSchemaURL is the $id of the embedded core schema; it never needs to
// resolve over the network; it is only used as a resource key.
const coreSchemaURL = "https://stroppy.io/dsl/core.schema.json"

// Kind selects which document shape (and thus which core.schema.json
// $defs entry) a YAML source is validated against.
type Kind int

const (
	// Cluster selects the cluster.yaml document shape.
	Cluster Kind = iota
	// Workflow selects the workflow.yaml document shape.
	Workflow
	// Component selects the component.yaml document shape (also used for
	// workload fragments, which share the same shape).
	Component
)

// compiledCore holds the core schema plus its three per-Kind dispatch
// targets, compiled once from the embedded core.schema.json.
type compiledCore struct {
	core      *jsonschema.Schema
	cluster   *jsonschema.Schema
	workflow  *jsonschema.Schema
	component *jsonschema.Schema
}

var (
	coreOnce   sync.Once
	coreParsed compiledCore
	errCore    error
)

// mustCompileCore compiles the embedded core.schema.json exactly once and
// panics on failure: a broken embedded schema is a programmatic bug in this
// package, never a user-input problem.
func mustCompileCore() compiledCore {
	coreOnce.Do(func() {
		coreParsed, errCore = compileCore()
	})
	if errCore != nil {
		panic(fmt.Sprintf("dsl/schema: compile embedded core.schema.json: %v", errCore))
	}
	return coreParsed
}

func compileCore() (compiledCore, error) {
	var doc any
	if err := json.Unmarshal(coreSchemaJSON, &doc); err != nil {
		return compiledCore{}, fmt.Errorf("parse embedded core.schema.json: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(coreSchemaURL, doc); err != nil {
		return compiledCore{}, fmt.Errorf("add core.schema.json resource: %w", err)
	}

	rootSchema, err := compiler.Compile(coreSchemaURL)
	if err != nil {
		return compiledCore{}, fmt.Errorf("compile core.schema.json: %w", err)
	}
	clusterSchema, err := compiler.Compile(coreSchemaURL + "#/$defs/cluster")
	if err != nil {
		return compiledCore{}, fmt.Errorf("compile $defs/cluster: %w", err)
	}
	workflowSchema, err := compiler.Compile(coreSchemaURL + "#/$defs/workflow")
	if err != nil {
		return compiledCore{}, fmt.Errorf("compile $defs/workflow: %w", err)
	}
	componentSchema, err := compiler.Compile(coreSchemaURL + "#/$defs/component")
	if err != nil {
		return compiledCore{}, fmt.Errorf("compile $defs/component: %w", err)
	}

	return compiledCore{
		core:      rootSchema,
		cluster:   clusterSchema,
		workflow:  workflowSchema,
		component: componentSchema,
	}, nil
}

// MustCore returns the compiled core schema (the embedded core.schema.json
// resource as a whole). It panics if the embedded schema fails to compile,
// since that can only happen due to a programmatic bug in this package, not
// due to user input.
func MustCore() *jsonschema.Schema {
	return mustCompileCore().core
}

// kindSchema returns the compiled per-Kind subschema (core.schema.json's
// $defs.cluster / $defs.workflow / $defs.component).
func kindSchema(kind Kind) *jsonschema.Schema {
	c := mustCompileCore()
	switch kind {
	case Cluster:
		return c.cluster
	case Workflow:
		return c.workflow
	case Component:
		return c.component
	default:
		panic(fmt.Sprintf("dsl/schema: unknown Kind %d", int(kind)))
	}
}

// Validate validates YAML source against the schema for kind: the composed
// schema if non-nil (built by Task 6's composer, tailored to a specific
// provider/document), else the matching core.schema.json subschema. Every
// problem is reported as a diagnostic in the returned diag.List (never as a
// Go error) with Path set to path and the JSON instance location folded
// into the message, so a caller sees every schema violation in one pass.
func Validate(kind Kind, path string, src []byte, composed *jsonschema.Schema) diag.List {
	var diags diag.List

	var raw any
	if err := yaml.Unmarshal(src, &raw); err != nil {
		diags.Errorf(path, diag.Pos{}, "yaml parse: %v", err)
		return diags
	}
	instance := normalize(raw)

	sch := composed
	if sch == nil {
		sch = kindSchema(kind)
	}

	if err := sch.Validate(instance); err != nil {
		addValidationErrors(&diags, path, err)
	}

	return diags
}

// normalize recursively converts a value decoded by gopkg.in/yaml.v3's
// yaml.Unmarshal into `any` into a shape github.com/santhosh-tekuri/jsonschema/v6
// accepts as a JSON instance: map[any]any -> map[string]any (yaml.v3
// usually already produces map[string]any for mapping nodes decoded into
// `any`, but this normalizes defensively rather than relying on that), with
// the same conversion applied recursively through slices and nested maps.
func normalize(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = normalize(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[fmt.Sprintf("%v", k)] = normalize(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = normalize(vv)
		}
		return out
	default:
		return v
	}
}

// addValidationErrors flattens a jsonschema.Validate error into diag.List
// entries, one per leaf schema-violation, each carrying the JSON instance
// location (e.g. "/machines/db/count") folded into the message.
func addValidationErrors(diags *diag.List, path string, err error) {
	var valErr *jsonschema.ValidationError
	if !errors.As(err, &valErr) {
		diags.Errorf(path, diag.Pos{}, "schema: %v", err)
		return
	}

	out := valErr.BasicOutput()
	if len(out.Errors) == 0 {
		diags.Errorf(path, diag.Pos{}, "%s", formatUnitMessage(*out))
		return
	}
	for _, unit := range out.Errors {
		diags.Errorf(path, diag.Pos{}, "%s", formatUnitMessage(unit))
	}
}

func formatUnitMessage(unit jsonschema.OutputUnit) string {
	loc := unit.InstanceLocation
	if loc == "" {
		loc = "/"
	}
	if unit.Error == nil {
		return loc
	}
	return fmt.Sprintf("%s: %s", loc, unit.Error.String())
}
