package valkey

type Config struct {
	Addresses []string `mapstructure:"addresses" validate:"required,dive,hostname_port"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password" validate:"required"`
}
