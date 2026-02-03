package install

import (
	"context"
	"fmt"

	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install/postgres/oriole"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install/postgres/vanilla"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install/stroppy"
)

type Config struct {
	DefaultPostgresVersion    string `mapstructure:"default_postgres_version" default:"15"`
	DefaultPostgresPort       int32  `mapstructure:"default_postgres_port" default:"5432"`
	DefaultPostgresListenAddr string `mapstructure:"default_postgres_listen_addr" default:"*"`
	DefaultPostgresUsername   string `mapstructure:"default_postgres_username" default:"postgres"`
	DefaultPostgresPassword   string `mapstructure:"default_postgres_password" default:"postgres"`

	DefaultStroppyVersion     string `mapstructure:"default_stroppy_version" default:"v2.0.0"`
	DefaultStroppyInstallPath string `mapstructure:"default_stroppy_path" default:"/usr/bin"`
}

const (
	DefaultPostgresVersion    = "17"
	DefaultPostgresPort       = 5432
	DefaultPostgresListenAddr = "*"
	DefaultPostgresPassword   = "postgres"
	DefaultPostgresUsername   = "postgres"

	DefaultStroppyVersion     = "v2.0.0"
	DefaultStroppyInstallPath = "/usr/bin"
)

func DefaultConfig() *Config {
	return &Config{
		DefaultPostgresVersion:    DefaultPostgresVersion,
		DefaultPostgresPort:       DefaultPostgresPort,
		DefaultPostgresListenAddr: DefaultPostgresListenAddr,
		DefaultPostgresPassword:   DefaultPostgresPassword,
		DefaultPostgresUsername:   DefaultPostgresUsername,

		DefaultStroppyVersion:     DefaultStroppyVersion,
		DefaultStroppyInstallPath: DefaultStroppyInstallPath,
	}
}

func (c *Config) PostgresUrlByIp(ip string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d",
		c.DefaultPostgresUsername,
		c.DefaultPostgresPassword,
		ip,
		c.DefaultPostgresPort,
	)
}

type Installer struct {
	config *Config
}

func New(config *Config) *Installer {
	return &Installer{
		config: config,
	}
}

func (i *Installer) InstallPostgres(ctx context.Context, params *hatchet.InstallPostgresParams) error {
	if params.GetEnableOrioledb() {
		return oriole.InstallAndConfigure(ctx, oriole.Config{
			Version:         params.GetVersion(),
			Port:            int(i.config.DefaultPostgresPort),
			ListenAddresses: i.config.DefaultPostgresListenAddr,
			Password:        i.config.DefaultPostgresPassword,
			Settings:        params.GetOrioledbSettings(),
		})
	}
	return vanilla.InstallAndConfigure(ctx, vanilla.Config{
		Version:         params.GetVersion(),
		Port:            int(i.config.DefaultPostgresPort),
		ListenAddresses: i.config.DefaultPostgresListenAddr,
		Password:        i.config.DefaultPostgresPassword,
		Settings:        params.GetSettings(),
	})
}

func (i *Installer) InstallStroppy(ctx context.Context, params *hatchet.RunStroppyParams) error {
	return stroppy.Install(ctx, stroppy.Config{
		Version:     defaults.StringOrDefault(params.GetVersion(), i.config.DefaultStroppyVersion),
		InstallPath: defaults.StringOrDefault(params.GetBinaryPath(), i.config.DefaultStroppyInstallPath),
	})
}
