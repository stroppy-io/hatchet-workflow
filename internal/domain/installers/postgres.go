package installers

import (
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

type PostgresConfig struct {
	Username                  string
	Password                  string
	Database                  string
	Version                   embeddedpostgres.PostgresVersion
	Port                      uint32
	DataPath                  string
	RuntimePath               string
	AdditionalStartParameters map[string]string
}

type postgresInstaller struct {
	postgresConfig embeddedpostgres.Config
	postgres       *embeddedpostgres.EmbeddedPostgres
}

func PostgresInstaller(config *PostgresConfig) Software {
	return &postgresInstaller{
		postgresConfig: embeddedpostgres.DefaultConfig().
			Username(config.Username).
			Password(config.Password).
			Database(config.Database).
			Version(config.Version).
			Port(config.Port).
			DataPath(config.DataPath). // для персистентных данных
			RuntimePath(config.RuntimePath).
			StartTimeout(45 * time.Second).
			StartParameters(config.AdditionalStartParameters),
	}
}

func (p *postgresInstaller) Install() error {
	p.postgres = embeddedpostgres.NewDatabase(p.postgresConfig)
	return nil
}

func (p *postgresInstaller) Start() error {
	return p.postgres.Start()
}
