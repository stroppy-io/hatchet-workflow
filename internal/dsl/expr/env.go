// Package expr provides the CEL environments used to typecheck and evaluate
// the ${{ ... }} expressions embedded in a stroppy YAML-DSL recipe bundle.
//
// Two environments exist because of the visibility rule from the pivot spec
// (§5.3 / §10): components and workloads write CEL in `requires:` and may
// reference only the domain view of a machine (no provider-specific data);
// providers write CEL in `capabilities.constraints:` and additionally see
// their own opaque `ext` map. The rule is enforced structurally: ComponentEnv
// registers MachineView (which has no Ext field), ProviderEnv registers
// ProviderMachineView (domain fields + Ext). An expression that reaches for
// `machine.ext` inside a ComponentEnv therefore fails at Check (typecheck)
// time — see TestVisibilityRule.
package expr

import (
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"google.golang.org/protobuf/types/known/structpb"
)

// DiskView is the CEL-visible projection of a machine's disk.
type DiskView struct {
	Path   string `cel:"path"`
	SizeGb int64  `cel:"size_gb"`
}

// MachineView is the CEL-visible projection of a single provisioned machine,
// restricted to domain fields. It is registered in ComponentEnv only: it has
// no Ext field, so `machine.ext` cannot typecheck against it.
type MachineView struct {
	IP     string     `cel:"ip"`
	RAMGb  int64      `cel:"ram_gb"`
	CPU    int64      `cel:"cpu"`
	DiskGb int64      `cel:"disk_gb"`
	Disks  []DiskView `cel:"disks"`
}

// MachineGroupView is the CEL-visible projection of a named group of
// machines (e.g. the `db` role in a topology).
type MachineGroupView struct {
	Count    int64         `cel:"count"`
	Machines []MachineView `cel:"machines"`
}

// ProviderMachineView is the CEL-visible projection of a machine as seen by
// a provider's own `capabilities.constraints:` expressions: the same domain
// fields as MachineView, plus the provider's opaque Ext data.
//
// Two deviations from the shape sketched in the task brief
// (`struct { MachineView; Ext map[string]any }`), both forced by the
// installed cel-go v0.29.0 ext.NativeTypes implementation and verified
// empirically (see task-7-report.md):
//
//  1. No Go struct embedding of MachineView. ext.NativeTypes, when
//     configured with ParseStructTags (required so CEL sees the spec's
//     snake_case field names instead of Go's exported field names), only
//     looks at a struct's immediate fields — it does not promote fields
//     through an embedded/anonymous struct the way Go's own field
//     resolution or json.Marshal would. An embedded MachineView would
//     surface in CEL as a single nested field (`machine.<tag on the
//     embedded field>.cpu`), not as promoted top-level fields
//     (`machine.cpu`), which is what the spec's provider constraint
//     examples require (cel-go's own ext.TestNativeStructEmbedded shows the
//     same nested-not-promoted behavior). ProviderMachineView therefore
//     duplicates MachineView's fields flatly so `machine.cpu`, `machine.ip`,
//     ... and `machine.ext...` all resolve directly off `machine`.
//  2. Ext is *structpb.Struct, not map[string]any. ext.NativeTypes derives
//     a struct field's CEL type via reflection (convertToCelType), which
//     switches on reflect.Kind and has no case for reflect.Interface — so a
//     map[string]any (or []any) field's element kind (Interface) always
//     fails type resolution, and any expression selecting that field
//     (`machine.ext`) fails to compile with "undefined field 'ext'"
//     regardless of the tag name. *structpb.Struct is a proto.Message, and
//     ext.NativeTypes' pointer-to-proto branch maps it to the well-known
//     type google.protobuf.Struct, which cel-go's standard checker already
//     understands as a dyn-valued string-keyed map — the correct supported
//     way to carry arbitrary provider-specific JSON-like data through CEL
//     native types in this version. NewProviderExt converts an
//     already-decoded map[string]any (e.g. ast.MachineGroup.Ext) into this
//     shape.
type ProviderMachineView struct {
	IP     string           `cel:"ip"`
	RAMGb  int64            `cel:"ram_gb"`
	CPU    int64            `cel:"cpu"`
	DiskGb int64            `cel:"disk_gb"`
	Disks  []DiskView       `cel:"disks"`
	Ext    *structpb.Struct `cel:"ext"`
}

// NewProviderExt converts a decoded YAML ext block (e.g.
// ast.MachineGroup.Ext) into the *structpb.Struct shape ProviderMachineView
// needs to expose it to CEL. Values must be JSON-representable (the same
// constraint structpb.NewStruct imposes).
func NewProviderExt(m map[string]any) (*structpb.Struct, error) {
	return structpb.NewStruct(m)
}

// nativeTypesOpt registers the given view structs as CEL native types, with
// struct-tag based field naming enabled so CEL sees the spec's snake_case
// names (e.g. "ram_gb") rather than Go's exported field names.
func nativeTypesOpt(types ...any) cel.EnvOption {
	args := append([]any{}, types...)
	args = append(args, ext.ParseStructTags(true))
	return ext.NativeTypes(args...)
}

// objType returns the cel.Type for a view struct registered via
// nativeTypesOpt. ext.NativeTypes names registered types
// "<last package path segment>.<StructName>"; both view structs below live
// in this package ("expr"), so the alias is always "expr".
func objType(name string) *cel.Type {
	return cel.ObjectType(fmt.Sprintf("expr.%s", name))
}

// baseOpts are the CEL extensions shared by both environments: the standard
// library (arithmetic, comparisons, has/map/filter/size macros, ...) plus
// string/list helpers (join, etc.) used by the spec's examples, e.g.
// `machines.db.machines.map(m, m.ip).join(",")`.
func baseOpts() []cel.EnvOption {
	return []cel.EnvOption{
		ext.Strings(),
		ext.Lists(),
	}
}

// ComponentEnv builds the CEL environment used to typecheck and evaluate
// component/workload `requires:` expressions. Per the visibility rule, it
// exposes only domain data: inputs, the current target group, all machine
// groups by role, the current machine (no ext), and the include matrix.
func ComponentEnv() (*cel.Env, error) {
	opts := append(baseOpts(),
		nativeTypesOpt(
			reflect.TypeOf(MachineView{}),
			reflect.TypeOf(MachineGroupView{}),
			reflect.TypeOf(DiskView{}),
		),
		cel.Variable("inputs", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("target", objType("MachineGroupView")),
		cel.Variable("machines", cel.MapType(cel.StringType, objType("MachineGroupView"))),
		cel.Variable("machine", objType("MachineView")),
		cel.Variable("matrix", cel.MapType(cel.StringType, cel.StringType)),
	)
	return cel.NewEnv(opts...)
}

// ProviderEnv builds the CEL environment used to typecheck and evaluate
// provider `capabilities.constraints:` expressions. Per the visibility rule,
// it exposes the domain machine view plus the provider's own opaque ext map
// via ProviderMachineView.
func ProviderEnv() (*cel.Env, error) {
	opts := append(baseOpts(),
		nativeTypesOpt(
			reflect.TypeOf(ProviderMachineView{}),
			reflect.TypeOf(DiskView{}),
		),
		cel.Variable("machine", objType("ProviderMachineView")),
	)
	return cel.NewEnv(opts...)
}
