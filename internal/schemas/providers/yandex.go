package providers

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// YandexProviderSchema is the Yandex Cloud (Terraform) provider settings.
// Populated per tenant (TenantSettings.providers) — credentials, region/network,
// the disk-type catalog, compute platform and quota limits the cluster schema
// validates machines against.
//
//schemapbgen:name YandexProviderConfig
func YandexProviderSchema() *schemapb.Schema {
	return schemapb.NewSchema(schemaNs, nameYandex, schemaVer).
		Descr("Yandex Cloud provider settings: credentials, region/network, "+
			"disk-type catalog, compute platform and quota limits.").
		Fields(
			// --- identity / credentials ---
			schemapb.Str(FieldCloudID).Required().Title("Cloud ID"),
			schemapb.Str(FieldFolderID).Required().Title("Folder ID"),
			schemapb.Str(FieldToken).Secret().Title("IAM/OAuth token").
				Desc("Sensitive — write-only, supplied per tenant."),

			// --- region / zones ---
			schemapb.List(FieldZones, schemapb.Str(FieldZones)).
				MinItems(1).MaxItems(8).Title("Available zones").
				Desc("e.g. ru-central1-a, ru-central1-b, ru-central1-d."),
			schemapb.Str(FieldDefaultZone).Title("Default zone"),

			// --- compute ---
			utils.StrEnum(FieldPlatformID, PlatformIDValues...).
				Default(PlatformStandardV3).Title("Compute platform"),
			schemapb.Str(FieldImageID).Title("Base image ID"),

			// --- disk-type catalog ---
			schemapb.List(FieldDiskTypes, utils.StrEnum(FieldDiskTypes, DiskTypeValues...)).
				MinItems(1).Title("Allowed disk types").
				Desc("The provider's disk-class catalog; machines pick from this set."),

			// --- network ---
			schemapb.Str(FieldNetworkID).Title("Network ID"),
			schemapb.Str(FieldSubnetCIDR).Default("10.10.0.0/24").Title("Subnet CIDR"),
			schemapb.Bool(FieldAssignPublicIP).Default(true).Title("Assign public IP"),
			schemapb.Bool(FieldSoftwareAcceleratedNetwork).Default(false).
				Title("Software-accelerated network"),

			// --- ssh ---
			schemapb.Str(FieldSSHUser).Default("stroppy").Title("SSH user"),
			schemapb.Str(FieldSSHPublicKey).Title("SSH public key"),

			// --- quota limits (cluster schema checks machines against these) ---
			schemapb.Object(FieldLimits,
				schemapb.Int32(FieldMaxNodes).Gte(0).Default(0).
					Title("Max nodes").Desc("0 = unlimited."),
				schemapb.Int32(FieldMaxCoresPerNode).Gte(0).Default(0).Title("Max cores/node"),
				schemapb.Int32(FieldMaxMemoryGBPerNode).Gte(0).Default(0).Unit("GB").
					Title("Max memory/node"),
				schemapb.Int32(FieldMaxDiskGBPerNode).Gte(0).Default(0).Unit("GB").
					Title("Max disk/node"),
			).Title("Quota limits"),
		).MustBuild()
}
