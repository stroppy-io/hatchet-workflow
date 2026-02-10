package install

import (
	"fmt"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/database"
)

type postgresInstaller struct {
	postgresConfig embeddedpostgres.Config
	postgres       *embeddedpostgres.EmbeddedPostgres
}

const (
	DefaultPostgresDataPath    = "/var/lib/postgresql/data"
	DefaultPostgresRuntimePath = "/usr/lib/postgresql"
	DefaultPostgresPort        = 5432
	DefaultPostgresDatabase    = "postgres"
	DefaultPostgresUsername    = "postgres"
	DefaultPostgresPassword    = "postgres"
)

func version(config database.Postgres_Instance_Version) embeddedpostgres.PostgresVersion {
	switch config {
	case database.Postgres_Instance_VERSION_18:
		return embeddedpostgres.V18
	case database.Postgres_Instance_VERSION_17:
		return embeddedpostgres.V17
	case database.Postgres_Instance_VERSION_16:
		return embeddedpostgres.V16
	case database.Postgres_Instance_VERSION_15:
		return embeddedpostgres.V15
	case database.Postgres_Instance_VERSION_14:
		return embeddedpostgres.V14
	default:
		return embeddedpostgres.V17
	}
}

func PostgresInstaller(config *database.Postgres_Instance) Installer {
	return &postgresInstaller{
		postgresConfig: embeddedpostgres.DefaultConfig().
			Username(defaults.StringOrDefault(config.GetUsername(), DefaultPostgresUsername)).
			Password(defaults.StringOrDefault(config.GetPassword(), DefaultPostgresPassword)).
			Database(defaults.StringOrDefault(config.GetDatabase(), DefaultPostgresDatabase)).
			Version(version(config.GetVersion())).
			Port(defaults.Uint32OrDefault(config.GetPort(), DefaultPostgresPort)).
			DataPath(DefaultPostgresDataPath).
			RuntimePath(DefaultPostgresRuntimePath).
			StartTimeout(45 * time.Second).
			StartParameters(config.GetPostgresqlConf()),
	}
}

func PostgresConnectionString(db *database.Postgres_Instance, host string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		defaults.StringOrDefault(db.GetUsername(), DefaultPostgresUsername),
		defaults.StringOrDefault(db.GetPassword(), DefaultPostgresPassword),
		host,
		defaults.Uint32OrDefault(db.GetPort(), DefaultPostgresPort),
		defaults.StringOrDefault(db.GetDatabase(), DefaultPostgresDatabase),
	)
}

func (p *postgresInstaller) Install() error {
	p.postgres = embeddedpostgres.NewDatabase(p.postgresConfig)
	return nil
}

func (p *postgresInstaller) Start() error {
	return p.postgres.Start()
}
