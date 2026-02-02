package install

import (
	"context"

	"github.com/stroppy-io/hatchet-workflow/internal/core/defaults"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install/postgres/oriole"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install/postgres/vanilla"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/install/stroppy"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Config struct {
	DefaultPostgresVersion    string `mapstructure:"default_postgres_version" default:"15"`
	DefaultPostgresPort       int32  `mapstructure:"default_postgres_port" default:"5432"`
	DefaultPostgresListenAddr string `mapstructure:"default_postgres_listen_addr" default:"*"`
	DefaultPostgresPassword   string `mapstructure:"default_postgres_password" default:"postgres"`

	DefaultStroppyInstallPath string `mapstructure:"default_stroppy_path" default:"/usr/bin"`
}

const (
	DefaultPostgresVersion    = "17"
	DefaultPostgresPort       = 5432
	DefaultPostgresListenAddr = "*"
	DefaultPostgresPassword   = "postgres"

	DefaultStroppyInstallPath = "/usr/bin"
)

func DefaultConfig() *Config {
	return &Config{
		DefaultPostgresVersion:    DefaultPostgresVersion,
		DefaultPostgresPort:       DefaultPostgresPort,
		DefaultPostgresListenAddr: DefaultPostgresListenAddr,
		DefaultPostgresPassword:   DefaultPostgresPassword,

		DefaultStroppyInstallPath: DefaultStroppyInstallPath,
	}
}

type Installer struct {
	*hatchet.UnimplementedInstallerServer
	config *Config
}

func New(config *Config) *Installer {
	return &Installer{
		UnimplementedInstallerServer: &hatchet.UnimplementedInstallerServer{},
		config:                       config,
	}
}

func (i *Installer) InstallPostgres(ctx context.Context, params *hatchet.InstallPostgresParams) (*emptypb.Empty, error) {
	if params.GetEnableOrioledb() {
		return nil, oriole.InstallAndConfigure(ctx, oriole.Config{
			Version:         params.GetVersion(),
			Port:            int(i.config.DefaultPostgresPort),
			ListenAddresses: i.config.DefaultPostgresListenAddr,
			Password:        i.config.DefaultPostgresPassword,
			Settings:        params.GetOrioledbSettings(),
		})
	}
	return nil, vanilla.InstallAndConfigure(ctx, vanilla.Config{
		Version:         params.GetVersion(),
		Port:            int(i.config.DefaultPostgresPort),
		ListenAddresses: i.config.DefaultPostgresListenAddr,
		Password:        i.config.DefaultPostgresPassword,
		Settings:        params.GetSettings(),
	})
}

func (i *Installer) InstallStroppy(ctx context.Context, params *hatchet.InstallStroppyParams) (*emptypb.Empty, error) {
	return nil, stroppy.Install(ctx, stroppy.Config{
		Version:     params.GetVersion(),
		InstallPath: defaults.StringOrDefault(params.GetBinaryPath(), i.config.DefaultStroppyInstallPath),
	})
}
