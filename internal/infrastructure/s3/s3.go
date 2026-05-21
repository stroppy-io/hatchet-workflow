package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gopherex/xlog"
	awslog "github.com/gopherex/xlog/contrib/libs/aws"
	"github.com/gopherex/xprobe"
)

// Config holds S3-compatible object storage connection settings.
type Config struct {
	Endpoint  string `mapstructure:"endpoint" validate:"required"`
	Region    string `mapstructure:"region" default:"us-east-1"`
	AccessKey string `mapstructure:"access_key" validate:"required"`
	SecretKey string `mapstructure:"secret_key" validate:"required"`
	Bucket    string `mapstructure:"bucket" validate:"required"`
	UseSSL    bool   `mapstructure:"use_ssl"`
}

// NewClient constructs an S3-compatible client from cfg.
// UsePathStyle is enabled to support MinIO and other S3-compatible stores.
func NewClient(cfg *Config, logger *xlog.Logger) (*s3sdk.Client, xprobe.Probe) {
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	endpoint := fmt.Sprintf("%s://%s", scheme, cfg.Endpoint)

	awsCfg := aws.Config{
		Region: cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		),
		BaseEndpoint: aws.String(endpoint),
		Logger:       awslog.New(logger),
	}
	client := s3sdk.NewFromConfig(awsCfg, func(o *s3sdk.Options) {
		o.UsePathStyle = true
	})
	return client, xprobe.FromError(func(ctx context.Context) error {
		logger.Trace("Pinging S3 server")
		_, err := client.HeadBucket(ctx, &s3sdk.HeadBucketInput{
			Bucket: aws.String(cfg.Bucket),
		})
		return err
	})
}

// EnsureBucket checks if the bucket exists and creates it if not.
func EnsureBucket(ctx context.Context, client *s3sdk.Client, bucket string) error {
	_, err := client.HeadBucket(ctx, &s3sdk.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		return nil
	}

	_, err = client.CreateBucket(ctx, &s3sdk.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("s3: create bucket %q: %w", bucket, err)
	}
	return nil
}
