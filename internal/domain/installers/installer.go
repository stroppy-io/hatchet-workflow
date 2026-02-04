package installers

type SoftwareConfig interface{}

type Software interface {
	Install() error
	Start() error
}

type SoftwareFactory[T SoftwareConfig] interface {
	Create(config T) Software
}

func InstallSoftware[T SoftwareConfig](factory SoftwareFactory[T], config T) error {
	software := factory.Create(config)
	return software.Install()
}

func StartSoftware[T SoftwareConfig](factory SoftwareFactory[T], config T) error {
	software := factory.Create(config)
	return software.Start()
}

func InstallAndStartSoftware[T SoftwareConfig](factory SoftwareFactory[T], config T) error {
	if err := InstallSoftware(factory, config); err != nil {
		return err
	}
	return StartSoftware(factory, config)
}

//type PostgresConfig struct {

//
//type Config struct {
//	DefaultPostgresVersion    string `mapstructure:"default_postgres_version" default:"15"`
//	DefaultPostgresPort       int32  `mapstructure:"default_postgres_port" default:"5432"`
//	DefaultPostgresListenAddr string `mapstructure:"default_postgres_listen_addr" default:"*"`
//	DefaultPostgresUsername   string `mapstructure:"default_postgres_username" default:"postgres"`
//	DefaultPostgresPassword   string `mapstructure:"default_postgres_password" default:"postgres"`
//
//	DefaultStroppyVersion     string `mapstructure:"default_stroppy_version" default:"v2.0.0"`
//	DefaultStroppyInstallPath string `mapstructure:"default_stroppy_path" default:"/usr/bin"`
//}
//
//const (
//	DefaultPostgresVersion    = "17"
//	DefaultPostgresPort       = 5432
//	DefaultPostgresListenAddr = "*"
//	DefaultPostgresPassword   = "postgres"
//	DefaultPostgresUsername   = "postgres"
//
//	DefaultStroppyVersion     = "v2.0.0"
//	DefaultStroppyInstallPath = "/usr/bin"
//)
//
//func DefaultConfig() *Config {
//	return &Config{
//		DefaultPostgresVersion:    DefaultPostgresVersion,
//		DefaultPostgresPort:       DefaultPostgresPort,
//		DefaultPostgresListenAddr: DefaultPostgresListenAddr,
//		DefaultPostgresPassword:   DefaultPostgresPassword,
//		DefaultPostgresUsername:   DefaultPostgresUsername,
//
//		DefaultStroppyVersion:     DefaultStroppyVersion,
//		DefaultStroppyInstallPath: DefaultStroppyInstallPath,
//	}
//}
//
//func (c *Config) PostgresUrlByIp(ip string) string {
//	return fmt.Sprintf(
//		"postgres://%s:%s@%s:%d",
//		c.DefaultPostgresUsername,
//		c.DefaultPostgresPassword,
//		ip,
//		c.DefaultPostgresPort,
//	)
//}
//
//type Installer struct {
//	config *Config
//}
//
//func New(config *Config) *Installer {
//	return &Installer{
//		config: config,
//	}
//}
