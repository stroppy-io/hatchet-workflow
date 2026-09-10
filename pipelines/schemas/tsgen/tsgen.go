// Package tsgen renders TypeScript value types for schemapb schemas — the
// shape a form holds and the API accepts for a field marked `x-schema`.
//
// Mapping (protoJSON wire shapes of schemapb Value):
//
//	int32/uint32/float/double → number; int64/uint64 → string | number
//	(protoJSON encodes 64-bit as strings; the TS engine accepts both)
//	bool → boolean; string → string; bytes → string (base64)
//	duration → string ("5s"); timestamp → string (RFC 3339)
//	choice → union of option literals (or the underlying type when `open`)
//	list → T[]; object → nested interface; map → Record<string, T>
//	oneOf → discriminated union on the discriminator key
//	ref → the named def's interface; computed → readonly field of its result type
//	json → unknown
//
// Optional fields (not required) are emitted with `?`; nullable adds `| null`.
package tsgen

import (
	"fmt"
	"sort"
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// Generate renders one .ts module for the schema: the root interface named
// after the schema plus one interface per def and per nested object.
func Generate(s *schemapb.Schema) string {
	g := &gen{defs: map[string]string{}}
	root := TypeName(s.GetId())
	g.header(s)
	// defs first so nested refs resolve by name
	defNames := make([]string, 0, len(s.GetDefs()))
	for n := range s.GetDefs() {
		defNames = append(defNames, n)
	}
	sort.Strings(defNames)
	for _, n := range defNames {
		g.defs[n] = root + pascal(n)
	}
	for _, n := range defNames {
		g.iface(g.defs[n], s.GetDefs()[n].GetFields(), "def "+n)
	}
	g.iface(root, s.GetFields(), "root")
	g.flush()
	return g.out.String()
}

// TypeName derives the exported TS type name: cfg.postgresql.conf@16 →
// CfgPostgresqlConf16.
func TypeName(id *schemapb.SchemaIdentity) string {
	pub := ids.Public(id)
	name, ver, _ := strings.Cut(pub, "@")
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '.' || r == '_' || r == '-' })
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(pascal(p))
	}
	// version: "16" → 16, "11.4" → 11_4 (keeps 11.4 and 1.14 apart)
	sb.WriteString(strings.ReplaceAll(ver, ".", "_"))
	return sb.String()
}

type gen struct {
	out     strings.Builder
	pending []string // nested interfaces discovered while rendering
	defs    map[string]string
	emitted map[string]int // interface name → times used, for de-duplication
}

// unique returns name, or name2/name3… when the same nested name was
// already emitted (two different objects under equally named fields).
func (g *gen) unique(name string) string {
	if g.emitted == nil {
		g.emitted = map[string]int{}
	}
	g.emitted[name]++
	if n := g.emitted[name]; n > 1 {
		return fmt.Sprintf("%s%d", name, n)
	}
	return name
}

// childName names the interface of a nested object: parent + field, or
// parent + "Item" for a list element (which has no field name).
func childName(parent string, f *schemapb.Schema_Field) string {
	if f.GetName() == "" {
		return parent + "Item"
	}
	return parent + pascal(f.GetName())
}

func (g *gen) header(s *schemapb.Schema) {
	fmt.Fprintf(&g.out, "// GENERATED from schemapb schema %s — do not edit.\n", ids.Public(s.GetId()))
	if d := s.GetDescription(); d != "" {
		fmt.Fprintf(&g.out, "// %s\n", strings.ReplaceAll(d, "\n", "\n// "))
	}
	g.out.WriteString("\n")
}

func (g *gen) flush() {
	for len(g.pending) > 0 {
		p := g.pending[0]
		g.pending = g.pending[1:]
		g.out.WriteString(p)
	}
}

func (g *gen) iface(name string, fields []*schemapb.Schema_Field, note string) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "/** %s */\nexport interface %s {\n", note, name)
	for _, f := range fields {
		if f.GetName() == "" {
			continue
		}
		if t := f.GetTitle(); t != "" || f.GetDescription() != "" {
			sb.WriteString("  /**")
			if t != "" {
				sb.WriteString(" " + t + ".")
			}
			if d := f.GetDescription(); d != "" {
				sb.WriteString(" " + strings.ReplaceAll(d, "*/", "* /"))
			}
			if u := f.GetUnit(); u != "" {
				sb.WriteString(" [" + u + "]")
			}
			sb.WriteString(" */\n")
		}
		mod := ""
		if f.GetComputed() != nil || f.GetImmutable() {
			mod = "readonly "
		}
		opt := "?"
		if f.GetRequired() {
			opt = ""
		}
		typ := g.typeOf(name, f)
		if f.GetNullable() {
			typ += " | null"
		}
		fmt.Fprintf(&sb, "  %s%s%s: %s;\n", mod, prop(f.GetName()), opt, typ)
	}
	sb.WriteString("}\n\n")
	g.pending = append(g.pending, sb.String())
}

func (g *gen) typeOf(parent string, f *schemapb.Schema_Field) string {
	switch {
	case f.GetInt32() != nil, f.GetUint32() != nil, f.GetFloat() != nil, f.GetDouble() != nil:
		return "number"
	case f.GetInt64() != nil, f.GetUint64() != nil:
		return "number | string"
	case f.GetBool() != nil:
		return "boolean"
	case f.GetString_() != nil:
		if in := f.GetString_().GetIn(); len(in) > 0 {
			return literals(in)
		}
		return "string"
	case f.GetBytes() != nil, f.GetDuration() != nil, f.GetTimestamp() != nil:
		return "string"
	case f.GetJson() != nil:
		return "unknown"
	case f.GetChoice() != nil:
		c := f.GetChoice()
		if c.GetOpen() || c.GetOptionsExpr() != "" || len(c.GetOptions()) == 0 {
			return "string"
		}
		vals := make([]string, 0, len(c.GetOptions()))
		for _, o := range c.GetOptions() {
			vals = append(vals, literal(o.GetValue()))
		}
		return strings.Join(vals, " | ")
	case f.GetList() != nil:
		items := f.GetList().GetItems()
		if len(items) == 1 {
			return "Array<" + g.typeOf(parent, items[0]) + ">"
		}
		parts := make([]string, 0, len(items))
		for _, it := range items {
			parts = append(parts, g.typeOf(parent, it))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case f.GetObject() != nil:
		name := g.unique(childName(parent, f))
		g.iface(name, f.GetObject().GetSchema().GetFields(), "object "+f.GetName())
		return name
	case f.GetMap() != nil:
		m := f.GetMap()
		if m.GetValueSchema() != nil {
			name := g.unique(childName(parent, f) + "Value")
			g.iface(name, m.GetValueSchema().GetFields(), "map value "+f.GetName())
			return "Record<string, " + name + ">"
		}
		if m.GetValueField() != nil {
			return "Record<string, " + g.typeOf(parent, m.GetValueField()) + ">"
		}
		return "Record<string, unknown>"
	case f.GetOneOf() != nil:
		o := f.GetOneOf()
		keys := make([]string, 0, len(o.GetVariants()))
		for k := range o.GetVariants() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			name := g.unique(childName(parent, f) + pascal(k))
			// The discriminator becomes a literal type; a variant that
			// declares it itself (strict variants do) is not repeated.
			fields := []*schemapb.Schema_Field{
				{Name: o.GetDiscriminator(), Required: true, Kind: &schemapb.Schema_Field_String_{String_: &schemapb.Schema_Field_String{In: []string{k}}}},
			}
			for _, vf := range o.GetVariants()[k].GetFields() {
				if vf.GetName() != o.GetDiscriminator() {
					fields = append(fields, vf)
				}
			}
			g.iface(name, fields, "variant "+k+" of "+f.GetName())
			parts = append(parts, name)
		}
		return strings.Join(parts, " | ")
	case f.GetRef() != nil:
		r := f.GetRef()
		if n := r.GetName(); n != "" {
			if t, ok := g.defs[n]; ok {
				return t
			}
			return "unknown /* def " + n + " */"
		}
		if id := r.GetId(); id != nil {
			return TypeName(id)
		}
		return "unknown"
	case f.GetComputed() != nil:
		switch f.GetComputed().GetResult() {
		case schemapb.Schema_Field_RESULT_TYPE_BOOL:
			return "boolean"
		case schemapb.Schema_Field_RESULT_TYPE_DOUBLE:
			return "number"
		case schemapb.Schema_Field_RESULT_TYPE_INT64, schemapb.Schema_Field_RESULT_TYPE_UINT64:
			return "number | string"
		case schemapb.Schema_Field_RESULT_TYPE_JSON:
			return "unknown"
		case schemapb.Schema_Field_RESULT_TYPE_STRING, schemapb.Schema_Field_RESULT_TYPE_DURATION,
			schemapb.Schema_Field_RESULT_TYPE_TIMESTAMP, schemapb.Schema_Field_RESULT_TYPE_BYTES,
			schemapb.Schema_Field_RESULT_TYPE_UNSPECIFIED:
			return "string"
		default:
			return "string"
		}
	}
	return "unknown"
}

func literals(in []string) string {
	parts := make([]string, 0, len(in))
	for _, s := range in {
		parts = append(parts, fmt.Sprintf("%q", s))
	}
	return strings.Join(parts, " | ")
}

func literal(v *schemapb.Value) string {
	switch {
	case v == nil:
		return "null"
	case v.GetKind() == nil:
		return "null"
	}
	switch k := v.GetKind().(type) {
	case *schemapb.Value_StringValue:
		return fmt.Sprintf("%q", k.StringValue)
	case *schemapb.Value_BoolValue:
		return fmt.Sprintf("%t", k.BoolValue)
	case *schemapb.Value_Int32Value:
		return fmt.Sprintf("%d", k.Int32Value)
	case *schemapb.Value_Int64Value:
		return fmt.Sprintf("%d", k.Int64Value)
	case *schemapb.Value_Uint32Value:
		return fmt.Sprintf("%d", k.Uint32Value)
	case *schemapb.Value_Uint64Value:
		return fmt.Sprintf("%d", k.Uint64Value)
	case *schemapb.Value_DoubleValue:
		return fmt.Sprintf("%g", k.DoubleValue)
	case *schemapb.Value_FloatValue:
		return fmt.Sprintf("%g", k.FloatValue)
	}
	return "unknown"
}

func prop(name string) string {
	for _, r := range name {
		if r != '_' && r != '$' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return fmt.Sprintf("%q", name)
		}
	}
	if name[0] >= '0' && name[0] <= '9' {
		return fmt.Sprintf("%q", name)
	}
	return name
}

func pascal(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == '.' || r == ' ' || r == '/' })
	var sb strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return sb.String()
}
