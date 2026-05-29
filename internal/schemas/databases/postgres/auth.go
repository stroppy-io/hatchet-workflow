package postgres

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// authSection configures pg_hba and SSL (pure DB).
func authSection() schemapb.FieldDef {
	return schemapb.Object(FieldAuth,
		utils.StrEnum(FieldAuthMethod, AuthMethodValues...).Default(AuthSCRAM).
			Title("Auth method").Desc("trust is bench-only / insecure."),
		utils.StrEnum(FieldPasswordEncryption, PasswordEncryptionValues...).Default(PwEncSCRAM).
			Title("password_encryption").Desc("Must match auth_method family."),
		schemapb.Bool(FieldSSL).Default(false).Title("SSL"),
		schemapb.Str(FieldListenAddresses).Default("*").Title("listen_addresses"),
		schemapb.List(FieldHBARules,
			schemapb.Object(FieldRule,
				utils.StrEnum(FieldType, HBATypeValues...).Default(HBAHost),
				schemapb.Str(FieldDatabase).Default("all"),
				schemapb.Str(FieldUser).Default("all"),
				schemapb.Str(FieldCIDR).Default("0.0.0.0/0"),
				utils.StrEnum(FieldMethod, HBAMethodValues...).Default(AuthSCRAM),
			),
		).Title("pg_hba rules"),
	).Title("Authentication & access")
}
