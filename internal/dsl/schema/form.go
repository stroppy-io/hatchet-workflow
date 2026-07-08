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
func ComposeFormSchema(namespace string, inputs, params *schemapb.Schema) (*schemapb.Schema, error) {
	fields := make([]schemapb.FieldDef, 0, len(inputs.GetFields())+1)
	for _, f := range inputs.GetFields() {
		fields = append(fields, f)
	}
	if params != nil && len(params.GetFields()) > 0 {
		fields = append(fields, schemapb.ObjectOf("provider", params))
	}

	return schemapb.NewSchema(namespace, "form", "1").Fields(fields...).Build()
}

// BakeForm validates and resolves filled form values against form, then
// seals them into a hashable Baked snapshot (the run identity). A nil
// FieldError slice/empty result means values passed validation cleanly;
// otherwise blocking errors leave baked nil while non-blocking warnings may
// accompany a non-nil baked -- see schemapb.Schema.Bake.
func BakeForm(form *schemapb.Schema, values map[string]any) (*schemapb.Baked, []*schemapb.FieldError, error) {
	if form == nil {
		return nil, nil, fmt.Errorf("bake form: nil schema")
	}
	baked, ferrs := form.Bake(values)
	return baked, ferrs, nil
}
