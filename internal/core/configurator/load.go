package configurator

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("configurator: read %s: %w", path, err)
	}
	expanded := os.ExpandEnv(string(raw))

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(expanded)); err != nil {
		return nil, fmt.Errorf("configurator: parse: %w", err)
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("configurator: unmarshal: %w", err)
	}
	return cfg, nil
}
