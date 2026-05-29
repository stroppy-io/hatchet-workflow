package mariadb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// authSection configures the default auth plugin, TLS posture, bind address and
// the declared user set (pure DB). Unlike MySQL 8.x, MariaDB has NO
// caching_sha2_password default — the historical default is mysql_native_password,
// with ed25519 as the modern strong option.
func authSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldAuth,
		utils.StrEnum(FieldDefaultAuthPlugin, AuthPluginValues...).Default(AuthNative).
			Title("Default authentication plugin").
			Desc("MariaDB default is mysql_native_password; ed25519 is the modern strong option. "+
				"(MariaDB has no caching_sha2_password.)"),
		utils.StrEnum(FieldSSLMode, SSLModeValues...).Default(SSLPreferred).
			Title("TLS posture").Desc("required maps to require_secure_transport=ON."),
		schemapb.Bool(FieldRequireSecureTransp).Default(false).
			When(utils.Eq(rp(pfx, FieldAuth, FieldSSLMode), SSLRequired)).
			Title("require_secure_transport"),
		schemapb.Str(FieldBindAddress).Default("0.0.0.0").Title("bind_address"),
		schemapb.List(FieldUsers,
			schemapb.Object(FieldUser,
				schemapb.Str(FieldName).Required().MinLen(1).Title("User name"),
				schemapb.Str(FieldHost).Default("%").Title("Host"),
				utils.StrEnum(FieldPlugin, AuthPluginValues...).Default(AuthNative).Title("Auth plugin"),
			),
		).Title("Users"),
	).Title("Authentication & access")
}
