

<a name="schemapb"></a>
# schemapb

## Table of Contents
- Messages
  - [schemapb.Baked](#schemapb-baked)
  - [schemapb.FieldError](#schemapb-fielderror)
  - [schemapb.FieldError.ParamsEntry](#schemapb-fielderror-paramsentry)
  - [schemapb.Schema](#schemapb-schema)
  - [schemapb.Schema.DefsEntry](#schemapb-schema-defsentry)
  - [schemapb.Schema.Filed](#schemapb-schema-filed)
  - [schemapb.Schema.Filed.Bool](#schemapb-schema-filed-bool)
  - [schemapb.Schema.Filed.Computed](#schemapb-schema-filed-computed)
  - [schemapb.Schema.Filed.Double](#schemapb-schema-filed-double)
  - [schemapb.Schema.Filed.Duration](#schemapb-schema-filed-duration)
  - [schemapb.Schema.Filed.Enum](#schemapb-schema-filed-enum)
  - [schemapb.Schema.Filed.Enum.ValuesEntry](#schemapb-schema-filed-enum-valuesentry)
  - [schemapb.Schema.Filed.Float](#schemapb-schema-filed-float)
  - [schemapb.Schema.Filed.Int32](#schemapb-schema-filed-int32)
  - [schemapb.Schema.Filed.Int64](#schemapb-schema-filed-int64)
  - [schemapb.Schema.Filed.List](#schemapb-schema-filed-list)
  - [schemapb.Schema.Filed.Object](#schemapb-schema-filed-object)
  - [schemapb.Schema.Filed.OneOf](#schemapb-schema-filed-oneof)
  - [schemapb.Schema.Filed.OneOf.VariantsEntry](#schemapb-schema-filed-oneof-variantsentry)
  - [schemapb.Schema.Filed.Ref](#schemapb-schema-filed-ref)
  - [schemapb.Schema.Filed.ResultType](#schemapb-schema-filed-resulttype)
  - [schemapb.Schema.Filed.Rule](#schemapb-schema-filed-rule)
  - [schemapb.Schema.Filed.Severity](#schemapb-schema-filed-severity)
  - [schemapb.Schema.Filed.String](#schemapb-schema-filed-string)
  - [schemapb.Schema.Filed.String.StringFormat](#schemapb-schema-filed-string-stringformat)
  - [schemapb.Schema.Filed.Timestamp](#schemapb-schema-filed-timestamp)
  - [schemapb.Schema.Filed.UInt32](#schemapb-schema-filed-uint32)
  - [schemapb.Schema.Filed.UInt64](#schemapb-schema-filed-uint64)
  - [schemapb.SchemaIdentity](#schemapb-schemaidentity)

<a name="schemapb-messages"></a>
## Messages

<a name="schemapb-baked"></a>
### schemapb.Baked

<pre>
Baked is a sealed result: the final values together with the schema they were
validated and resolved against. It is a self-contained, immutable snapshot —
"these exact values for this exact schema, not to be changed". Unlike Filled
(which only references a schema and is mutable), Baked embeds the full schema
and the frozen values, so it is portable and verifiable on its own.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>schema</td>
<td><a href="#schemapb-schema">schemapb.Schema</a></td>
<td><pre>
The schema these values were baked against (embedded, self-contained).<br>

json_name: schema
go_name: Schema</pre></td>
</tr><tr>
<td>values</td>
<td><a href="../google/protobuf/README.md#google-protobuf-struct">google.protobuf.Struct</a></td>
<td><pre>
The final, resolved values — frozen.<br>

json_name: values
go_name: Values</pre></td>
</tr>
</table>



<a name="schemapb-fielderror"></a>
### schemapb.FieldError

<pre>
FieldError is a single validation failure returned by the server-side
validator (and reusable to render server errors on the client).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>code</td>
<td>string</td>
<td><pre>
Stable machine code identifying the kind of failure.<br>

json_name: code
go_name: Code</pre></td>
</tr><tr>
<td>field</td>
<td>string</td>
<td><pre>
Field name or dotted path that failed.<br>

json_name: field
go_name: Field</pre></td>
</tr><tr>
<td>message</td>
<td>string</td>
<td><pre>
Human-readable failure message.<br>

json_name: message
go_name: Message</pre></td>
</tr><tr>
<td>params</td>
<td><a href="#schemapb-fielderror-paramsentry">schemapb.FieldError.ParamsEntry</a></td>
<td><pre>
Template parameters for client-side i18n of the error message.<br>

json_name: params
go_name: Params</pre></td>
</tr><tr>
<td>rule_id</td>
<td>string</td>
<td><pre>
Id of the rule that failed, if any.<br>

json_name: ruleId
go_name: RuleId</pre></td>
</tr><tr>
<td>severity</td>
<td><a href="#schemapb-schema-filed-severity">schemapb.Schema.Filed.Severity</a></td>
<td><pre>
Severity of the failure.<br>

json_name: severity
go_name: Severity</pre></td>
</tr>
</table>



<a name="schemapb-fielderror-paramsentry"></a>
### schemapb.FieldError.ParamsEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="schemapb-schema"></a>
### schemapb.Schema

<pre>
Schema is a runtime, dynamic form descriptor. The server builds a Schema and
ships it to the frontend, which renders a form from it.

Validation has two layers:
  - per-kind structured rules (gt/gte/lt/lte/in/pattern/...) for the common,
    single-field cases. Ergonomic to author and introspectable by the
    renderer (e.g. slider bounds, step).
  - Filed.rules: CEL expressions, for cross-field / conditional validation.

Both layers run through one engine: structured rules are lowered to CEL at
load time, then evaluated by cel-go (server, authoritative) and cel-es
(client, live UX) from the same expression. Rendering is described by a
separate Form (see ui.proto) that references fields by path; the Schema
itself carries no UI.

The PGV (validate.rules) options below validate the descriptor ITSELF (that a
Schema is well-formed: names set, expressions non-empty, a kind chosen), not
the runtime form values it describes.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>coerce</td>
<td>bool</td>
<td><pre>
If true, scalar string inputs are coerced to the field's kind before
validation (numeric kinds parse the string as a number; bool parses
"true"/"false"; enum parses an integer string). Only applied when the
current value is a string but the field's kind expects a different type
and parsing succeeds; unparseable strings are left as-is so the type
error is reported normally.<br>

json_name: coerce
go_name: Coerce</pre></td>
</tr><tr>
<td>defs</td>
<td><a href="#schemapb-schema-defsentry">schemapb.Schema.DefsEntry</a></td>
<td><pre>
Named reusable sub-schemas ($defs). Fields of kind Ref resolve their
value against the named def found here. Meaningful only on the root
schema; defs inside nested schemas are ignored by the engine.<br>

json_name: defs
go_name: Defs</pre></td>
</tr><tr>
<td>description</td>
<td>string</td>
<td><pre>
Human description of the schema.<br>

json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>fields</td>
<td><a href="#schemapb-schema-filed">schemapb.Schema.Filed</a></td>
<td><pre>
The fields that make up the form.<br>

json_name: fields
go_name: Fields</pre></td>
</tr><tr>
<td>id</td>
<td><a href="#schemapb-schemaidentity">schemapb.SchemaIdentity</a></td>
<td><pre>
Stable identity of this schema. Required: the validator rejects a schema
whose id is unset or whose name is empty.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>max_properties</td>
<td>uint64</td>
<td><pre>
Maximum number of properties allowed in the values map.<br>

json_name: maxProperties
go_name: MaxProperties</pre></td>
</tr><tr>
<td>min_properties</td>
<td>uint64</td>
<td><pre>
Minimum number of properties that must be present in the values map.<br>

json_name: minProperties
go_name: MinProperties</pre></td>
</tr><tr>
<td>rules</td>
<td><a href="#schemapb-schema-filed-rule">schemapb.Schema.Filed.Rule</a></td>
<td><pre>
Form-wide invariant rules (`root` only, no `this`).<br>

json_name: rules
go_name: Rules</pre></td>
</tr><tr>
<td>strict</td>
<td>bool</td>
<td><pre>
If true, any key in the values map that is not a declared field name
is an error (code "unknown_field").<br>

json_name: strict
go_name: Strict</pre></td>
</tr>
</table>



<a name="schemapb-schema-defsentry"></a>
### schemapb.Schema.DefsEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td><a href="#schemapb-schema">schemapb.Schema</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed"></a>
### schemapb.Schema.Filed

<pre>
Filed is one field of the form.

Per-field evaluation order (each stage reads the form as resolved by the
previous one):
  1. when         — gate: if false the field is INACTIVE and every stage
                    below is skipped; the field is treated as ABSENT (its
                    value, if any, is preserved but ignored). For a
                    container kind (Object/OneOf/List/Ref) the WHOLE
                    subtree is gated.
  2. normalize    — map the field's own value.
  3. Computed     — derive the value from `root`.
  4. options_expr / count_expr — dynamic Enum options / List length.
  5. rules + kind constraints — required/nullable, gt/lt/in/pattern, ...

Renderer contract: an inactive field (when=false) MUST NOT be rendered.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>bool</td>
<td><a href="#schemapb-schema-filed-bool">schemapb.Schema.Filed.Bool</a></td>
<td><pre>
Bool field.<br>

json_name: bool
go_name: Bool</pre></td>
</tr><tr>
<td>computed</td>
<td><a href="#schemapb-schema-filed-computed">schemapb.Schema.Filed.Computed</a></td>
<td><pre>
Computed (derived) field.<br>

json_name: computed
go_name: Computed</pre></td>
</tr><tr>
<td>deprecated</td>
<td>bool</td>
<td><pre>
If true, this field is deprecated; renderers may warn. Purely informative.<br>

json_name: deprecated
go_name: Deprecated</pre></td>
</tr><tr>
<td>description</td>
<td>string</td>
<td><pre>
Human description of the field.<br>

json_name: description
go_name: Description</pre></td>
</tr><tr>
<td>double</td>
<td><a href="#schemapb-schema-filed-double">schemapb.Schema.Filed.Double</a></td>
<td><pre>
Double field.<br>

json_name: double
go_name: Double</pre></td>
</tr><tr>
<td>duration</td>
<td><a href="#schemapb-schema-filed-duration">schemapb.Schema.Filed.Duration</a></td>
<td><pre>
Duration field.<br>

json_name: duration
go_name: Duration</pre></td>
</tr><tr>
<td>enum</td>
<td><a href="#schemapb-schema-filed-enum">schemapb.Schema.Filed.Enum</a></td>
<td><pre>
Enum field.<br>

json_name: enum
go_name: Enum</pre></td>
</tr><tr>
<td>examples</td>
<td><a href="../google/protobuf/README.md#google-protobuf-value">google.protobuf.Value</a></td>
<td><pre>
Example values for documentation/rendering. Purely informative.<br>

json_name: examples
go_name: Examples</pre></td>
</tr><tr>
<td>float</td>
<td><a href="#schemapb-schema-filed-float">schemapb.Schema.Filed.Float</a></td>
<td><pre>
Float field.<br>

json_name: float
go_name: Float</pre></td>
</tr><tr>
<td>group</td>
<td>string</td>
<td><pre>
Informative section label for grouping fields (e.g. "WAL"). Purely
informative — the validation/compute engine ignores it.<br>

json_name: group
go_name: Group</pre></td>
</tr><tr>
<td>immutable</td>
<td>bool</td>
<td><pre>
If true, the value is system-fixed: it equals `default` and cannot be
changed. The validator forces it to `default` and rejects any
different submitted value; renderers show it disabled. A "system
value" is therefore immutable + default.<br>

json_name: immutable
go_name: Immutable</pre></td>
</tr><tr>
<td>int32</td>
<td><a href="#schemapb-schema-filed-int32">schemapb.Schema.Filed.Int32</a></td>
<td><pre>
Int32 field.<br>

json_name: int32
go_name: Int32</pre></td>
</tr><tr>
<td>int64</td>
<td><a href="#schemapb-schema-filed-int64">schemapb.Schema.Filed.Int64</a></td>
<td><pre>
Int64 field.<br>

json_name: int64
go_name: Int64</pre></td>
</tr><tr>
<td>list</td>
<td><a href="#schemapb-schema-filed-list">schemapb.Schema.Filed.List</a></td>
<td><pre>
List field.<br>

json_name: list
go_name: List</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
Field name; the key under which the value lives in the form.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>normalize</td>
<td>string</td>
<td><pre>
Normalize expression: maps the field's own value before validation.
`this` = current value, `root` = whole form. Returns the new value.
Applied after defaults/coercion and before Computed evaluation and
validation.<br>

json_name: normalize
go_name: Normalize</pre></td>
</tr><tr>
<td>nullable</td>
<td>bool</td>
<td><pre>
If true, the value may be null/empty.<br>

json_name: nullable
go_name: Nullable</pre></td>
</tr><tr>
<td>object</td>
<td><a href="#schemapb-schema-filed-object">schemapb.Schema.Filed.Object</a></td>
<td><pre>
Nested object field.<br>

json_name: object
go_name: Object</pre></td>
</tr><tr>
<td>one_of</td>
<td><a href="#schemapb-schema-filed-oneof">schemapb.Schema.Filed.OneOf</a></td>
<td><pre>
Discriminated union field.<br>

json_name: oneOf
go_name: OneOf</pre></td>
</tr><tr>
<td>ref</td>
<td><a href="#schemapb-schema-filed-ref">schemapb.Schema.Filed.Ref</a></td>
<td><pre>
Reference to a named definition in the root schema's defs.<br>

json_name: ref
go_name: Ref</pre></td>
</tr><tr>
<td>required</td>
<td>bool</td>
<td><pre>
If true, the value must be present.<br>

json_name: required
go_name: Required</pre></td>
</tr><tr>
<td>rules</td>
<td><a href="#schemapb-schema-filed-rule">schemapb.Schema.Filed.Rule</a></td>
<td><pre>
Cross-field CEL validation rules (`this` is bound to this field).<br>

json_name: rules
go_name: Rules</pre></td>
</tr><tr>
<td>secret</td>
<td>bool</td>
<td><pre>
If true, the value is sensitive (e.g. a password). Purely informative.<br>

json_name: secret
go_name: Secret</pre></td>
</tr><tr>
<td>string</td>
<td><a href="#schemapb-schema-filed-string">schemapb.Schema.Filed.String</a></td>
<td><pre>
String field.<br>

json_name: string
go_name: String_</pre></td>
</tr><tr>
<td>timestamp</td>
<td><a href="#schemapb-schema-filed-timestamp">schemapb.Schema.Filed.Timestamp</a></td>
<td><pre>
Timestamp field.<br>

json_name: timestamp
go_name: Timestamp</pre></td>
</tr><tr>
<td>title</td>
<td>string</td>
<td><pre>
Informative human title for the field. Purely informative — engine ignores it.<br>

json_name: title
go_name: Title</pre></td>
</tr><tr>
<td>uint32</td>
<td><a href="#schemapb-schema-filed-uint32">schemapb.Schema.Filed.UInt32</a></td>
<td><pre>
UInt32 field.<br>

json_name: uint32
go_name: Uint32</pre></td>
</tr><tr>
<td>uint64</td>
<td><a href="#schemapb-schema-filed-uint64">schemapb.Schema.Filed.UInt64</a></td>
<td><pre>
UInt64 field.<br>

json_name: uint64
go_name: Uint64</pre></td>
</tr><tr>
<td>unit</td>
<td>string</td>
<td><pre>
Informative unit of the value, e.g. "MB" or "ms". Purely informative —
the engine ignores it; it is for renderers/serializers.<br>

json_name: unit
go_name: Unit</pre></td>
</tr><tr>
<td>when</td>
<td>string</td>
<td><pre>
Conditional gate: an expr boolean over `root` (the whole form as
map<string, dyn>). When it evaluates to false the field is INACTIVE —
the validator skips it ENTIRELY (no required/nullable, no rules, no
kind constraints, no Computed/normalize) and treats it as ABSENT
regardless of any value present in `values`. For a container kind the
whole subtree is gated. Inactive fields do not count toward
min/max_properties and their value key never raises a strict
"unknown_field". Their value is NOT deleted, so it reappears if the
field becomes active again. Renderers MUST hide an inactive field.
`this` is NOT bound (a field's own value must not gate its
existence). Empty/absent => always active. A non-bool result is a
runtime error.<br>

json_name: when
go_name: When</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-bool"></a>
### schemapb.Schema.Filed.Bool

<pre>
Bool field kind.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>bool</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>bool</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-computed"></a>
### schemapb.Schema.Filed.Computed

<pre>
Computed field kind: a value derived from other values, not entered by
the user. The server (and the client, for live UX) evaluates `expr`
and writes the result under this field's name, so other expressions
and validation rules can read it via `root`.

Evaluation is dependency-ordered: a Computed field may reference inputs
and other Computed fields. Cycles are rejected at schema-validation
time. Referencing a value that is absent at evaluation time is an
error (surfaced as a FieldError on this field's path).

`expr` must be a pure, deterministic expression (no now()/rand) so the
server and client agree.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>expr</td>
<td>string</td>
<td><pre>
Expression producing the value. Reads `root` (inputs + already
computed values). Bound the same way as Rule expressions.<br>

json_name: expr
go_name: Expr</pre></td>
</tr><tr>
<td>result</td>
<td><a href="#schemapb-schema-filed-resulttype">schemapb.Schema.Filed.ResultType</a></td>
<td><pre>
Result type, used to marshal the value back into the form. The
field kind it names (Float/Int32/.../Duration) selects the wire
type; structured constraints on it are ignored for output but may
still be used to document/round the result. Optional: if unset the
server infers the type from the checked expression.<br>

json_name: result
go_name: Result</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-double"></a>
### schemapb.Schema.Filed.Double

<pre>
Double field kind and its single-field constraints.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>double</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>double</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td>double</td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td>double</td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>in</td>
<td>double</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>lt</td>
<td>double</td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td>double</td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr><tr>
<td>multiple_of</td>
<td>double</td>
<td><pre>
Divisibility: quotient value/multiple_of must be whole.<br>

json_name: multipleOf
go_name: MultipleOf</pre></td>
</tr><tr>
<td>not_in</td>
<td>double</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-duration"></a>
### schemapb.Schema.Filed.Duration

<pre>
Duration field kind with optional range bounds.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>default</td>
<td><a href="../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td><a href="../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td><a href="../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>lt</td>
<td><a href="../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td><a href="../google/protobuf/README.md#google-protobuf-duration">google.protobuf.Duration</a></td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-enum"></a>
### schemapb.Schema.Filed.Enum

<pre>
Enum field kind: an integer value with human labels.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>default</td>
<td>int32</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>defined_only</td>
<td>bool</td>
<td><pre>
If true, value must be one of the keys in values.<br>

json_name: definedOnly
go_name: DefinedOnly</pre></td>
</tr><tr>
<td>in</td>
<td>int32</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>not_in</td>
<td>int32</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr><tr>
<td>options_expr</td>
<td>string</td>
<td><pre>
expr over `root` returning a list of allowed integer values. When
set it REPLACES the static allowed set (values/in/not_in/
defined_only) for validation and supplies the option list to
renderers. The submitted value must be a member of the result,
else FieldError code "enum_not_allowed". Empty/absent => use the
static values. The result must be a list; a non-list is a runtime
error. Use cases: db versions by kind, zones by region.<br>

json_name: optionsExpr
go_name: OptionsExpr</pre></td>
</tr><tr>
<td>values</td>
<td><a href="#schemapb-schema-filed-enum-valuesentry">schemapb.Schema.Filed.Enum.ValuesEntry</a></td>
<td><pre>
Allowed enum values: integer -> human label.<br>

json_name: values
go_name: Values</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-enum-valuesentry"></a>
### schemapb.Schema.Filed.Enum.ValuesEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>int32</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td>string</td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-float"></a>
### schemapb.Schema.Filed.Float

<pre>
Float field kind and its single-field constraints.
gt = exclusive min (> 0), gte = inclusive min (>= 1).
lt = exclusive max, lte = inclusive max.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>float</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>float</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td>float</td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td>float</td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>in</td>
<td>float</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>lt</td>
<td>float</td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td>float</td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr><tr>
<td>multiple_of</td>
<td>float</td>
<td><pre>
Divisibility: quotient value/multiple_of must be whole.<br>

json_name: multipleOf
go_name: MultipleOf</pre></td>
</tr><tr>
<td>not_in</td>
<td>float</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-int32"></a>
### schemapb.Schema.Filed.Int32

<pre>
Int32 field kind and its single-field constraints.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>int32</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>int32</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td>int32</td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td>int32</td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>in</td>
<td>int32</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>lt</td>
<td>int32</td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td>int32</td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr><tr>
<td>multiple_of</td>
<td>int32</td>
<td><pre>
Divisibility: value % multiple_of must be 0.<br>

json_name: multipleOf
go_name: MultipleOf</pre></td>
</tr><tr>
<td>not_in</td>
<td>int32</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-int64"></a>
### schemapb.Schema.Filed.Int64

<pre>
Int64 field kind and its single-field constraints.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>int64</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>int64</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td>int64</td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td>int64</td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>in</td>
<td>int64</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>lt</td>
<td>int64</td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td>int64</td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr><tr>
<td>multiple_of</td>
<td>int64</td>
<td><pre>
Divisibility: value % multiple_of must be 0.<br>

json_name: multipleOf
go_name: MultipleOf</pre></td>
</tr><tr>
<td>not_in</td>
<td>int64</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-list"></a>
### schemapb.Schema.Filed.List

<pre>
List field kind: a repeated value described by its element field(s).
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>count_expr</td>
<td>string</td>
<td><pre>
expr over `root` returning a non-negative int: the exact number
of items the list must have. Renderers generate that many item
slots. The list length must equal the result, else FieldError
code "list_count_mismatch". Empty/absent => length bounded only by
min_items/max_items. A non-int or negative result is a runtime
error. Each item is still validated by the item schema; inside an
item's rules the item's zero-based position is bound as `index`.
Use case: per-machine settings where N = replicas + 1.<br>

json_name: countExpr
go_name: CountExpr</pre></td>
</tr><tr>
<td>items</td>
<td><a href="#schemapb-schema-filed">schemapb.Schema.Filed</a></td>
<td><pre>
Element field definition(s) describing list items.<br>

json_name: items
go_name: Items</pre></td>
</tr><tr>
<td>max_items</td>
<td>uint64</td>
<td><pre>
Maximum number of items.<br>

json_name: maxItems
go_name: MaxItems</pre></td>
</tr><tr>
<td>min_items</td>
<td>uint64</td>
<td><pre>
Minimum number of items.<br>

json_name: minItems
go_name: MinItems</pre></td>
</tr><tr>
<td>unique</td>
<td>bool</td>
<td><pre>
If true, items must be unique.<br>

json_name: unique
go_name: Unique</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-object"></a>
### schemapb.Schema.Filed.Object

<pre>
Object field kind: a nested sub-form described by its own Schema.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>schema</td>
<td><a href="#schemapb-schema">schemapb.Schema</a></td>
<td><pre>
Nested object schema.<br>

json_name: schema
go_name: Schema</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-oneof"></a>
### schemapb.Schema.Filed.OneOf

<pre>
OneOf field kind: a discriminated union. The value must be an object
whose discriminator property selects a variant schema.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>discriminator</td>
<td>string</td>
<td><pre>
Property name in the value object that selects the variant.<br>

json_name: discriminator
go_name: Discriminator</pre></td>
</tr><tr>
<td>variants</td>
<td><a href="#schemapb-schema-filed-oneof-variantsentry">schemapb.Schema.Filed.OneOf.VariantsEntry</a></td>
<td><pre>
Variant schemas keyed by the discriminator value.<br>

json_name: variants
go_name: Variants</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-oneof-variantsentry"></a>
### schemapb.Schema.Filed.OneOf.VariantsEntry

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>key</td>
<td>string</td>
<td><pre>
json_name: key
go_name: Key</pre></td>
</tr><tr>
<td>value</td>
<td><a href="#schemapb-schema">schemapb.Schema</a></td>
<td><pre>
json_name: value
go_name: Value</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-ref"></a>
### schemapb.Schema.Filed.Ref

<pre>
Ref field kind: the value is an object validated against another
schema. The target is selected one of two ways:
  - name: a key in the root schema's defs map (local composition;
    enables recursion — a def may Ref back to itself, terminating on
    the finite data).
  - id:   the SchemaIdentity of a separately-registered schema. The
    identity is PRESERVED on the node (renderers can show/link the
    target), and the referenced schema must be made resolvable —
    either present in the root defs under its identity key, or pulled
    in by Link(resolver) before validation. An id-ref to a schema not
    present in defs is an "unknown $ref" error at validate time.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>id</td>
<td><a href="#schemapb-schemaidentity">schemapb.SchemaIdentity</a></td>
<td><pre>
Identity of a registered schema (resolved via defs / Link).<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>name</td>
<td>string</td>
<td><pre>
Name of the def in the root schema's defs map.<br>

json_name: name
go_name: Name</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-resulttype"></a>
### schemapb.Schema.Filed.ResultType

<pre>
ResultType is the wire type a Computed expression yields.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>RESULT_TYPE_UNSPECIFIED</td>
<td><pre>
Unset: infer from the expression.
</pre></td>
</tr><tr>
<td>RESULT_TYPE_DOUBLE</td>
<td></td>
</tr><tr>
<td>RESULT_TYPE_INT64</td>
<td></td>
</tr><tr>
<td>RESULT_TYPE_UINT64</td>
<td></td>
</tr><tr>
<td>RESULT_TYPE_BOOL</td>
<td></td>
</tr><tr>
<td>RESULT_TYPE_STRING</td>
<td></td>
</tr><tr>
<td>RESULT_TYPE_DURATION</td>
<td></td>
</tr>
</table>

<a name="schemapb-schema-filed-rule"></a>
### schemapb.Schema.Filed.Rule

<pre>
Rule is a CEL validation expression for cross-field / conditional checks.
Context: `this` = this field's value (field-level rules only);
         `root` = whole form as map<string, dyn>, any field reachable.
expr must evaluate to bool; true means VALID.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>expr</td>
<td>string</td>
<td><pre>
CEL boolean expression; true means the value is valid.<br>

json_name: expr
go_name: Expr</pre></td>
</tr><tr>
<td>id</td>
<td>string</td>
<td><pre>
Stable rule id, for mapping errors back to rules.<br>

json_name: id
go_name: Id</pre></td>
</tr><tr>
<td>message</td>
<td>string</td>
<td><pre>
Message shown to the user when expr is false.<br>

json_name: message
go_name: Message</pre></td>
</tr><tr>
<td>severity</td>
<td><a href="#schemapb-schema-filed-severity">schemapb.Schema.Filed.Severity</a></td>
<td><pre>
Severity; defaults to ERROR.<br>

json_name: severity
go_name: Severity</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-severity"></a>
### schemapb.Schema.Filed.Severity

<pre>
Severity of a failed rule.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>SEVERITY_UNSPECIFIED</td>
<td><pre>
Unset: treated as ERROR.
</pre></td>
</tr><tr>
<td>ERROR</td>
<td><pre>
Blocks submit.
</pre></td>
</tr><tr>
<td>WARNING</td>
<td><pre>
Surfaced to the user but does not block submit.
</pre></td>
</tr>
</table>

<a name="schemapb-schema-filed-string"></a>
### schemapb.Schema.Filed.String

<pre>
String field kind and its single-field constraints.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>string</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>string</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>format</td>
<td><a href="#schemapb-schema-filed-string-stringformat">schemapb.Schema.Filed.String.StringFormat</a></td>
<td><pre>
Semantic format the value must conform to.<br>

json_name: format
go_name: Format</pre></td>
</tr><tr>
<td>in</td>
<td>string</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>len</td>
<td>uint64</td>
<td><pre>
Exact character length required.<br>

json_name: len
go_name: Len</pre></td>
</tr><tr>
<td>max_len</td>
<td>uint64</td>
<td><pre>
Maximum character length.<br>

json_name: maxLen
go_name: MaxLen</pre></td>
</tr><tr>
<td>min_len</td>
<td>uint64</td>
<td><pre>
Minimum character length.<br>

json_name: minLen
go_name: MinLen</pre></td>
</tr><tr>
<td>not_in</td>
<td>string</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr><tr>
<td>pattern</td>
<td>string</td>
<td><pre>
RE2 regular expression the value must match.<br>

json_name: pattern
go_name: Pattern</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-string-stringformat"></a>
### schemapb.Schema.Filed.String.StringFormat

<pre>
StringFormat enumerates well-known semantic string formats.
</pre>

<table>
<tr><th>Value</th><th>Description</th></tr>
<tr>
<td>STRING_FORMAT_UNSPECIFIED</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_EMAIL</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_URL</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_UUID</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_IPV4</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_IPV6</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_IP</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_HOSTNAME</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_DATE</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_TIME</td>
<td></td>
</tr><tr>
<td>STRING_FORMAT_DATETIME</td>
<td></td>
</tr>
</table>

<a name="schemapb-schema-filed-timestamp"></a>
### schemapb.Schema.Filed.Timestamp

<pre>
Timestamp field kind with optional range bounds.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>default</td>
<td><a href="../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td><a href="../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td><a href="../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>lt</td>
<td><a href="../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td><a href="../google/protobuf/README.md#google-protobuf-timestamp">google.protobuf.Timestamp</a></td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-uint32"></a>
### schemapb.Schema.Filed.UInt32

<pre>
UInt32 field kind and its single-field constraints.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>uint32</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>uint32</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td>uint32</td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td>uint32</td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>in</td>
<td>uint32</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>lt</td>
<td>uint32</td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td>uint32</td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr><tr>
<td>multiple_of</td>
<td>uint32</td>
<td><pre>
Divisibility: value % multiple_of must be 0.<br>

json_name: multipleOf
go_name: MultipleOf</pre></td>
</tr><tr>
<td>not_in</td>
<td>uint32</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr>
</table>



<a name="schemapb-schema-filed-uint64"></a>
### schemapb.Schema.Filed.UInt64

<pre>
UInt64 field kind and its single-field constraints.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>const</td>
<td>uint64</td>
<td><pre>
Value must equal exactly this.<br>

json_name: const
go_name: Const</pre></td>
</tr><tr>
<td>default</td>
<td>uint64</td>
<td><pre>
Default value used when the field is unset.<br>

json_name: default
go_name: Default</pre></td>
</tr><tr>
<td>gt</td>
<td>uint64</td>
<td><pre>
Exclusive minimum: value > gt.<br>

json_name: gt
go_name: Gt</pre></td>
</tr><tr>
<td>gte</td>
<td>uint64</td>
<td><pre>
Inclusive minimum: value >= gte.<br>

json_name: gte
go_name: Gte</pre></td>
</tr><tr>
<td>in</td>
<td>uint64</td>
<td><pre>
Value must be one of these.<br>

json_name: in
go_name: In</pre></td>
</tr><tr>
<td>lt</td>
<td>uint64</td>
<td><pre>
Exclusive maximum: value < lt.<br>

json_name: lt
go_name: Lt</pre></td>
</tr><tr>
<td>lte</td>
<td>uint64</td>
<td><pre>
Inclusive maximum: value <= lte.<br>

json_name: lte
go_name: Lte</pre></td>
</tr><tr>
<td>multiple_of</td>
<td>uint64</td>
<td><pre>
Divisibility: value % multiple_of must be 0.<br>

json_name: multipleOf
go_name: MultipleOf</pre></td>
</tr><tr>
<td>not_in</td>
<td>uint64</td>
<td><pre>
Value must not be any of these.<br>

json_name: notIn
go_name: NotIn</pre></td>
</tr>
</table>



<a name="schemapb-schemaidentity"></a>
### schemapb.SchemaIdentity

<pre>
SchemaIdentity is the stable, system-assigned identity of a Schema. A Schema
is addressed by this identity rather than a free-form name.
</pre>

<table>
<tr>
<th>Attribute</th>
<th>Type</th>
<th>Description</th>
</tr>
<tr>
<td>name</td>
<td>string</td>
<td><pre>
Schema name, unique within its namespace. Required.<br>

json_name: name
go_name: Name</pre></td>
</tr><tr>
<td>namespace</td>
<td>string</td>
<td><pre>
Grouping namespace, e.g. a product or team. Optional.<br>

json_name: namespace
go_name: Namespace</pre></td>
</tr><tr>
<td>version</td>
<td>string</td>
<td><pre>
Schema version, e.g. "v1" or a semver string. Optional.<br>

json_name: version
go_name: Version</pre></td>
</tr>
</table>

