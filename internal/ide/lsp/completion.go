package lsp

import (
	"context"
	"fmt"
	"strings"

	"github.com/stroppy-io/schemapb/schemapb"
	"go.lsp.dev/protocol"
	"google.golang.org/protobuf/encoding/protojson"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
	"github.com/stroppy-io/stroppy-cloud/internal/services/dsl"
)

// CompletionItems calls DslService.ComposedSchema over bundleFiles and
// converts its response into LSP CompletionItems.
//
// PLAN CORRECTION (see .superpowers/sdd/spc-t5-t7-report.md): the plan
// illustrated this as decoding ComposedSchemaResponse.SchemaJson as a plain
// JSON-Schema document (properties/required/enum). That is NOT what the
// deployed ComposedSchema returns — internal/services/dsl/service.go's real
// implementation protojson-marshals a *schemapb.Schema (see its own doc
// comment: "schema_json — protojson-сериализованная schemapb.Schema (не
// JSON Schema)"), and internal/services/dsl/service_test.go's
// TestComposedSchema_ReturnsSchemapbProtojson proves it by unmarshalling the
// response straight into a schemapb.Schema. This function follows the real
// contract (protojson.Unmarshal into *schemapb.Schema, walk GetFields())
// rather than the plan's JSON-Schema shape, which would fail to parse the
// actual response at all.
//
// Depth: every top-level field (workflow inputs, hoisted by
// schema.ComposeFormSchema — see form.go) plus one level under the
// "provider" object field (provider params, schema.DeriveProviderParamsSchemapb),
// surfaced as dotted "provider.<name>" items. This matches
// ComposeFormSchema's own two-level shape exactly (root fields, plus one
// nested "provider" object — form.go never nests deeper than that), so this
// is not an arbitrarily chosen depth limit: it is the full depth
// ComposedSchema's response can express today. A provider whose own
// Terraform variables are themselves objects/maps would need a third level;
// that is a real, disclosed gap (see the T5-T7 report), not built here
// because DeriveProviderParamsSchemapb's tf-variable-derived object/map
// fields are not exercised by any existing golden fixture to verify
// against, and inventing one risks silently drifting from the compiler.
func CompletionItems(ctx context.Context, svc *dsl.DslService, bundleFiles map[string][]byte) ([]protocol.CompletionItem, error) {
	resp, err := svc.ComposedSchema(ctx, &dslpb.ComposedSchemaRequest{Files: bundleFiles})
	if err != nil {
		return nil, err
	}

	var form schemapb.Schema
	if err := protojson.Unmarshal([]byte(resp.GetSchemaJson()), &form); err != nil {
		return nil, fmt.Errorf("unmarshal composed schema as schemapb.Schema: %w", err)
	}

	items := make([]protocol.CompletionItem, 0, len(form.GetFields()))
	for _, f := range form.GetFields() {
		items = append(items, completionItemForField("", f))
		if obj := f.GetObject(); obj != nil {
			for _, nested := range obj.GetSchema().GetFields() {
				items = append(items, completionItemForField(f.GetName()+".", nested))
			}
		}
	}
	return items, nil
}

func completionItemForField(prefix string, f *schemapb.Schema_Filed) protocol.CompletionItem {
	kind := fieldKindLabel(f)
	detail := kind
	if f.GetRequired() {
		detail += " (required)"
	}
	doc := f.GetDescription()
	if enum := f.GetEnum(); enum != nil && len(enum.GetValues()) > 0 {
		values := make([]string, 0, len(enum.GetValues()))
		for _, label := range enum.GetValues() {
			values = append(values, label)
		}
		enumLine := "allowed: " + strings.Join(values, ", ")
		if doc != "" {
			doc += "\n" + enumLine
		} else {
			doc = enumLine
		}
	}
	return protocol.CompletionItem{
		Label:         prefix + f.GetName(),
		Kind:          protocol.CompletionItemKindField,
		Detail:        detail,
		Documentation: doc,
		InsertText:    prefix + f.GetName(),
	}
}

// fieldKindLabel returns a short human-readable name for a schemapb field's
// oneof Kind — purely informative (LSP CompletionItem.Detail), mirroring the
// same switch schemapb's own render.go performs over the identical oneof
// (see that file's "switch f.GetKind().(type)"), so this label set cannot
// silently miss a kind the library itself doesn't already handle elsewhere.
func fieldKindLabel(f *schemapb.Schema_Filed) string {
	switch f.GetKind().(type) {
	case *schemapb.Schema_Filed_Float_:
		return "float"
	case *schemapb.Schema_Filed_Double_:
		return "double"
	case *schemapb.Schema_Filed_Int32_:
		return "int32"
	case *schemapb.Schema_Filed_Int64_:
		return "int64"
	case *schemapb.Schema_Filed_Uint32:
		return "uint32"
	case *schemapb.Schema_Filed_Uint64:
		return "uint64"
	case *schemapb.Schema_Filed_Bool_:
		return "bool"
	case *schemapb.Schema_Filed_String_:
		return "string"
	case *schemapb.Schema_Filed_Enum_:
		return "enum"
	case *schemapb.Schema_Filed_Duration_:
		return "duration"
	case *schemapb.Schema_Filed_Timestamp_:
		return "timestamp"
	case *schemapb.Schema_Filed_List_:
		return "list"
	case *schemapb.Schema_Filed_Object_:
		return "object"
	case *schemapb.Schema_Filed_Computed_:
		return "computed"
	case *schemapb.Schema_Filed_OneOf_:
		return "oneOf"
	case *schemapb.Schema_Filed_Ref_:
		return "ref"
	case *schemapb.Schema_Filed_Map_:
		return "map"
	default:
		return "unknown"
	}
}
