package run

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/dbconfig"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// BuildRenderedConfigs returns the per-component config files that the agent
// will write on each database node, rendered from the resolved RunConfig.
//
// Keys are stable identifiers shared with DatabaseConfig.RenderedConfigOverrides
// (e.g. "postgresql.conf:master"). The SPA seeds its textareas from this map
// and ships back any user-edited entries under the same keys.
//
// User-supplied overrides win: when a key is present in
// cfg.Database.RenderedConfigOverrides we surface that value verbatim so the
// preview shows what's actually going to land on the box.
func BuildRenderedConfigs(cfg *types.RunConfig) map[string]string {
	out := map[string]string{}
	db := cfg.Database
	overrides := db.RenderedConfigOverrides

	put := func(key, body string) {
		if v, ok := overrides[key]; ok {
			out[key] = v
			return
		}
		out[key] = body
	}

	switch db.Kind {
	case types.DatabasePostgres:
		if db.Postgres == nil {
			return out
		}
		put("postgresql.conf:master", dbconfig.RenderPostgresConf(dbconfig.RenderPostgresConfOpts{
			Version:       db.Version,
			Role:          "master",
			Options:       db.Postgres.MasterOptions,
			Patroni:       db.Postgres.Patroni,
			TotalMemoryMB: db.Postgres.Master.MemoryMB,
		}))
		if len(db.Postgres.Replicas) > 0 {
			r := db.Postgres.Replicas[0]
			put("postgresql.conf:replica", dbconfig.RenderPostgresConf(dbconfig.RenderPostgresConfOpts{
				Version:       db.Version,
				Role:          "replica",
				Options:       db.Postgres.ReplicaOptions,
				Patroni:       db.Postgres.Patroni,
				TotalMemoryMB: r.MemoryMB,
			}))
		}
		put("pg_hba.conf", dbconfig.PostgresPgHbaConf())

	case types.DatabaseMySQL:
		if db.MySQL == nil {
			return out
		}
		put("my.cnf:primary", dbconfig.RenderMySQLConf(dbconfig.RenderMySQLConfOpts{
			Version:       db.Version,
			Role:          "primary",
			SemiSync:      db.MySQL.SemiSync,
			GroupRepl:     db.MySQL.GroupRepl,
			Options:       db.MySQL.PrimaryOptions,
			TotalMemoryMB: db.MySQL.Primary.MemoryMB,
		}))
		if len(db.MySQL.Replicas) > 0 {
			r := db.MySQL.Replicas[0]
			put("my.cnf:replica", dbconfig.RenderMySQLConf(dbconfig.RenderMySQLConfOpts{
				Version:       db.Version,
				Role:          "replica",
				SemiSync:      db.MySQL.SemiSync,
				GroupRepl:     db.MySQL.GroupRepl,
				Options:       db.MySQL.ReplicaOptions,
				TotalMemoryMB: r.MemoryMB,
			}))
		}
	}

	return out
}
