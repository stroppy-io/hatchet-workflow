package schema

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"
)

// ComposeFormSchema merges a workflow/component's declared inputs with its
// provider params into a single launch-form schema: every inputs field is
// hoisted to the form's top level, and params (if any) are nested under a
// single "provider" object field via schemapb.ObjectOf.
//
// NOTE on re-adding already-built fields: schemapb/new.go already provides
// exactly the adapter this needs -- FieldDef is `interface{ Done() *Schema_Filed }`,
// and schemapb defines `func (f *Schema_Filed) Done() *Schema_Filed { return f }`
// (new.go, "Done lets a raw field satisfy FieldDef"). So a *Schema_Filed
// pulled from inputs.GetFields() already satisfies FieldDef and can be passed
// straight to .Fields(...) -- no local shim is needed.
// Both the composed root and (when present) the nested "provider" object are
// built strict (schemapb.SchemaB.Strict / schemapb.ObjectB.Strict): Bake
// rejects any key the schema didn't declare instead of silently letting it
// through to ApplyBakedInputs (defense in depth -- see baked_apply.go's own
// strict-by-declared-keys check, which guards the path that doesn't go
// through a BakeForm call at all, e.g. a Baked crafted directly by a client
// and handed straight to dsl.Compile).
func ComposeFormSchema(namespace string, inputs, params *schemapb.Schema) (*schemapb.Schema, error) {
	fields := make([]schemapb.FieldDef, 0, len(inputs.GetFields())+1)
	for _, f := range inputs.GetFields() {
		fields = append(fields, f)
	}
	if params != nil && len(params.GetFields()) > 0 {
		fields = append(fields, schemapb.ObjectOf("provider", params).Strict())
	}

	return schemapb.NewSchema(namespace, "form", "1").Strict().Fields(fields...).Build()
}

// BakeForm validates and resolves filled form values against form, then
// seals them into a hashable Baked snapshot (the run identity). A nil
// FieldError slice/empty result means values passed validation cleanly;
// otherwise blocking errors leave baked nil while non-blocking warnings may
// accompany a non-nil baked -- see schemapb.Schema.Bake.
//
// Numeric contract: Go callers MUST supply numeric field values as float64
// -- schemapb's numericCheck type-asserts the value to float64 and rejects
// anything else (int, int64, ...) as a type-mismatch FieldError, even for
// int/int64-kind fields. The live JSON (or YAML) -> structpb.Value ->
// map[string]any decoding path already yields float64 for every JSON
// number, so this only matters for values assembled directly in Go (e.g.
// tests).
func BakeForm(form *schemapb.Schema, values map[string]any) (*schemapb.Baked, []*schemapb.FieldError, error) {
	if form == nil {
		return nil, nil, fmt.Errorf("bake form: nil schema")
	}
	baked, ferrs := form.Bake(values)
	return baked, ferrs, nil
}
