package system

import (
	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas/ids"
)

// sizeNames are the T-shirt sizes, smallest first. The whole product speaks
// only these: a test picks a size per role, the size table turns it into a
// concrete machine.
// doc: OpenAPI `Size` (openapi/parts/10-components-common.yaml).
var sizeNames = []string{"XS", "S", "M", "L", "XL"}

// providerNames are the provider kinds the table is keyed by; they are named
// fields rather than free map keys because the set is closed.
// doc: OpenAPI `ProviderKind` (openapi/parts/30-tenant-settings.yaml).
var providerNames = []string{"yandex", "aws"}

// sizeSpec is one cell of the table: what a size means for one provider and
// one role family. It matches OpenAPI SizeSpec
// (openapi/parts/40-catalog.yaml) minus the redundant `size` property, which
// the key already carries.
func sizeSpec(name schemapb.FieldName) *schemapb.ObjectB {
	return schemapb.Object(name,
		schemapb.Int64("cpu").Title("vCPU").
			Desc("Cores the machine gets.").Gte(1).Lte(288).Required(),
		schemapb.Int64("memory_gb").Title("Memory").Unit("GB").
			Desc("RAM the machine gets.").Gte(1).Lte(4096).Required(),
		schemapb.Str("instance_type").Title("Instance type").
			Desc("Platform id (yandex, e.g. standard-v3) or EC2 instance type (aws, e.g. m7i.2xlarge).").
			MinLen(1).MaxLen(64).Required(),
		schemapb.Int64("default_disk_gb").Title("Default disk").Unit("GB").
			Desc("Data disk size when the test does not override it.").
			Gte(10).Lte(262144).Required(),
		schemapb.Str("disk_type").Title("Disk type").
			Desc("Provider disk type id: network-ssd / network-ssd-io-m3 (yandex), gp3 / io2 (aws).").
			Pattern(`^[a-z0-9][a-z0-9-]{0,31}$`).Required(),
	).Strict().Required()
}

// providerTable is one provider's half of the table: role family → XS..XL.
func providerTable(provider string) *schemapb.MapB {
	sizes := make([]schemapb.FieldDef, 0, len(sizeNames))
	for _, n := range sizeNames {
		sizes = append(sizes, sizeSpec(schemapb.FieldName(n)).
			Title(n).Desc("What size "+n+" means for this role family."))
	}

	return schemapb.Map(schemapb.FieldName(provider), sizes...).
		Strict().Title(provider).Group("Sizes").
		Desc("Keys are role families (db, proxy, runner, coordinator); each maps XS..XL to a machine.").
		MaxEntries(16)
}

// monotonic builds the CEL rule that a numeric property never shrinks as the
// size grows, across every role family of one provider.
func monotonic(provider, prop string) string {
	expr := `!("` + provider + `" in root) || root.` + provider + `.all(r, `

	for i := 1; i < len(sizeNames); i++ {
		if i > 1 {
			expr += " && "
		}

		expr += "root." + provider + "[r]." + sizeNames[i-1] + "." + prop +
			" <= root." + provider + "[r]." + sizeNames[i] + "." + prop
	}

	return expr + ")"
}

// Sizes is system.sizes@1 — the platform size table: provider → role family →
// size → concrete machine. It is the only place a T-shirt size becomes real
// hardware; the run compiler reads it and writes the result into the RunSpec.
//
// doc: STROPPY.MD §5 ("таблица size → (platform/instance type, cores, memory,
// disk) per provider живёт на сервере"), §16.4.
func Sizes() *schemapb.Schema {
	b := schemapb.NewSchema(ids.System("sizes", 1)).
		Descr("Platform size table: provider → role family → XS..XL → concrete machine.").
		Strict().Coerce()

	for _, p := range providerNames {
		b = b.Fields(providerTable(p))
		b = b.Rules(
			schemapb.Rule(monotonic(p, "cpu"),
				p+": cpu must not shrink as the size grows").ID(schemapb.RuleID(p+"-cpu-monotonic")),
			schemapb.Rule(monotonic(p, "memory_gb"),
				p+": memory must not shrink as the size grows").ID(schemapb.RuleID(p+"-memory-monotonic")),
			schemapb.Rule(monotonic(p, "default_disk_gb"),
				p+": the default disk should not shrink as the size grows").
				ID(schemapb.RuleID(p+"-disk-monotonic")).Warn(),
		)
	}

	return b.MustBuild()
}
