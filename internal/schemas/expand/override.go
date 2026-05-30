package expand

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/providers"
	"github.com/stroppy-io/stroppy-cloud/internal/schemas/types"
)

// OverrideSchema builds the provider-specific override-param schema the wizard
// shows for machines of the given role, computed from (provider × db kind) with
// option sets narrowed to the TENANT provider settings (`tenantProv` = the
// tenant's ProviderSettings values: disk_types, zones, platform_id). The result
// is a small schemapb form the user edits; an empty schema means "nothing to
// override" (e.g. docker). The abstract machine stays separate (its shape maps
// to Instance.machine_info; these override values map to Instance.provider_parms).
//
// dbKind/role are passed for upcoming DB-specific constraints (e.g. a YDB
// network-ssd-io-m3 disk forces the machine disk to a multiple of 93 GB, or
// mirror-3-dc widens zone choice). Those are a TODO; today only the tenant
// catalog narrows the options.
func OverrideSchema(providerType, dbKind, role string, tenantProv map[string]any) *schemapb.Schema {
	var fields []schemapb.FieldDef

	switch providerType {
	case ProviderYandex:
		disks := mapStrSlice(tenantProv, providers.FieldDiskTypes)
		zones := mapStrSlice(tenantProv, providers.FieldZones)
		platform := mapStr(tenantProv, providers.FieldPlatformID, providers.PlatformStandardV3)
		fields = providers.YandexOverrideFields(disks, zones, platform)
		// TODO(db-constraints): narrow/augment by dbKind+role (io-m3 %93, mirror-3-dc zones).
	case ProviderDocker:
		fields = providers.DockerOverrideFields()
	}

	b := schemapb.NewSchema(types.SchemaNs("override"), types.SchemaName(providerType+"-"+role), "1.0.0")
	if len(fields) > 0 {
		b = b.Fields(fields...)
	}
	return b.MustBuild()
}
