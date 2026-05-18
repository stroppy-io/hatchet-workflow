package configurator

import "time"

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Postgres    PostgresConfig    `mapstructure:"postgres"`
	Valkey      ValkeyConfig      `mapstructure:"valkey"`
	S3          S3Config          `mapstructure:"s3"`
	Victoria    VictoriaConfig    `mapstructure:"victoria"`
	Terraform   TerraformConfig   `mapstructure:"terraform"`
	Stroppy     StroppyConfig     `mapstructure:"stroppy"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Idempotency IdempotencyConfig `mapstructure:"idempotency"`
	Workers     WorkersConfig     `mapstructure:"workers"`
	Features    FeaturesConfig    `mapstructure:"features"`
	Log         LogConfig         `mapstructure:"log"`
}

type ServerConfig struct{ HTTPAddr string `mapstructure:"http_addr"` }
type PostgresConfig struct {
	DSN      string `mapstructure:"dsn"`
	MaxConns int    `mapstructure:"max_conns"`
}
type ValkeyConfig struct {
	Addr string `mapstructure:"addr"`
	DB   int    `mapstructure:"db"`
}
type S3Config struct {
	Endpoint     string `mapstructure:"endpoint"`
	Bucket       string `mapstructure:"bucket"`
	Region       string `mapstructure:"region"`
	AccessKeyEnv string `mapstructure:"access_key_env"`
	SecretKeyEnv string `mapstructure:"secret_key_env"`
	UsePathStyle bool   `mapstructure:"use_path_style"`
}
type VictoriaConfig struct {
	PushURL  string `mapstructure:"push_url"`
	QueryURL string `mapstructure:"query_url"`
}
type TerraformConfig struct {
	BinaryPath  string `mapstructure:"binary_path"`
	WorkdirRoot string `mapstructure:"workdir_root"`
}
type StroppyConfig struct {
	DefaultVersion string `mapstructure:"default_version"`
	BinariesDir    string `mapstructure:"binaries_dir"`
	ReleasesURL    string `mapstructure:"releases_url"`
	CommitsURL     string `mapstructure:"commits_url"`
}
type AuthConfig struct {
	JWTSecretEnv string        `mapstructure:"jwt_secret_env"`
	AccessTTL    time.Duration `mapstructure:"access_ttl"`
	RefreshTTL   time.Duration `mapstructure:"refresh_ttl"`
	// CookieSecure marks the refresh-token cookie Secure (HTTPS-only). Keep
	// false for local dev over plain HTTP; set true in production.
	CookieSecure bool `mapstructure:"cookie_secure"`
}
type IdempotencyConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	TTL     time.Duration `mapstructure:"ttl"`
}
type WorkersConfig struct {
	NodeWorkers     int           `mapstructure:"node_workers"`
	SchedulerTick   time.Duration `mapstructure:"scheduler_tick"`
	WebhookWorkers  int           `mapstructure:"webhook_workers"`
	RecoveryOnStart bool          `mapstructure:"recovery_on_start"`
}
type FeaturesConfig struct {
	InitialAdminEmail       string `mapstructure:"initial_admin_email"`
	InitialAdminPasswordEnv string `mapstructure:"initial_admin_password_env"`
}
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}
