package mariadb

import (
	"github.com/stroppy-io/schemapb/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/schemas/utils"
)

// proxySection configures an optional ProxySQL tier (CONFIG only — intent flag +
// ProxySQL settings). Whether ProxySQL is colocated or on a dedicated machine,
// its listen ports and the backend host list are placement/deployment decisions
// owned by the cluster schema.
func proxySection(pfx string) schemapb.FieldDef {
	return schemapb.Object(FieldProxy,
		schemapb.Bool(FieldUseProxy).Default(false).
			Title("Use ProxySQL").Desc("Front the cluster with ProxySQL for read/write split."),
		schemapb.Object(FieldProxySQL,
			schemapb.Str(FieldMonitorUser).Default("monitor").
				Title("Monitor user").Desc("ProxySQL backend health-check user."),
			schemapb.Int32(FieldMaxConnections).Gte(1).Default(2048).
				Title("Backend max_connections").Desc("Per-hostgroup backend connection cap."),
			schemapb.Bool(FieldQueryRulesFast).Default(true).
				Title("Fast routing rules").
				Desc("Generate default read/write split query rules (SELECT -> reader hostgroup)."),
			schemapb.List(FieldHostgroups,
				schemapb.Object(FieldHostgroup,
					schemapb.Int32(FieldHostgroupID).Gte(0).Required().Title("Hostgroup ID"),
					utils.StrEnum(FieldHostgroupRole, ProxyHostgroupRoleValues...).
						Default(HostgroupWriter).Title("Role"),
				),
			).Title("Hostgroups").
				Desc("writer/reader hostgroup map (typically writer=10, reader=20). "+
					"Backend addresses are wired at the cluster layer."),
		).When(utils.IsTrue(rp(pfx, FieldProxy, FieldUseProxy))).Title("ProxySQL"),
	).Title("Proxy intent")
}
