package s3

import (
	"fmt"
	"os"
)

type Config struct {
	Url             string `mapstructure:"url" default:"http://localhost:9000" validate:"required,url"`
	Region          string `mapstructure:"region" default:"us-east-1" validate:"required"`
	SecretAccessKey string `mapstructure:"secret_key" validate:"required"`
	AccessKeyId     string `mapstructure:"key_id" validate:"required"`
}

func (c Config) Validate() error {
	if c.Url == "" {
		return fmt.Errorf("url is required")
	}
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("secret access key is required")
	}
	if c.AccessKeyId == "" {
		return fmt.Errorf("access key id is required")
	}
	return nil
}

const (
	s3UrlKey             = "S3_URL"
	s3RegionKey          = "S3_REGION"
	s3SecretAccessKeyKey = "S3_SECRET_ACCESS_KEY"
	s3AccessKeyIdKey     = "S3_ACCESS_KEY_ID"
)

func NewConfigFromEnv() (*Config, error) {
	c := &Config{
		Url:             os.Getenv(s3UrlKey),
		Region:          os.Getenv(s3RegionKey),
		SecretAccessKey: os.Getenv(s3SecretAccessKeyKey),
		AccessKeyId:     os.Getenv(s3AccessKeyIdKey),
	}
	err := c.Validate()
	if err != nil {
		return nil, err
	}
	return c, nil
}
