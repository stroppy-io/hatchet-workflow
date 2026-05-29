package mysql

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// authSection configures the default auth plugin, TLS posture, bind address and
// the declared user set (pure DB). Concrete grants/credentials and listener
// wiring are handled downstream.
func authSection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldAuth,
		utils.StrEnum(FieldDefaultAuthPlugin, AuthPluginValues...).Default(AuthCachingSHA2).
			Title("default_authentication_plugin").
			Desc("caching_sha2_password is the MySQL 8.0+/8.4 default; "+
				"mysql_native_password is deprecated (and disabled by default in 8.4)."),
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
				utils.StrEnum(FieldPlugin, AuthPluginValues...).Default(AuthCachingSHA2).Title("Auth plugin"),
			),
		).Title("Users"),
	).Title("Authentication & access")
}
