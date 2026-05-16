package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	awsv2cfg "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/stroppy-io/stroppy-cloud/internal/core/configurator"
)

type Client struct {
	api     *awss3.Client
	presign *awss3.PresignClient
	bucket  string
}

func New(ctx context.Context, cfg configurator.S3Config) (*Client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket required")
	}
	awsCfg, err := awsv2cfg.LoadDefaultConfig(ctx,
		awsv2cfg.WithRegion(cfg.Region),
		awsv2cfg.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(os.Getenv(cfg.AccessKeyEnv), os.Getenv(cfg.SecretKeyEnv), "")),
	)
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	opts := []func(*awss3.Options){}
	if cfg.Endpoint != "" {
		opts = append(opts, func(o *awss3.Options) {
			o.BaseEndpoint = &cfg.Endpoint
			o.UsePathStyle = cfg.UsePathStyle
		})
	}
	api := awss3.NewFromConfig(awsCfg, opts...)
	return &Client{api: api, bucket: cfg.Bucket, presign: awss3.NewPresignClient(api)}, nil
}

func (c *Client) PutObject(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	out, err := c.api.PutObject(ctx, &awss3.PutObjectInput{Bucket: &c.bucket, Key: &key, Body: body, ContentType: &contentType})
	if err != nil {
		return "", err
	}
	if out.ETag == nil {
		return "", nil
	}
	return *out.ETag, nil
}

func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &awss3.GetObjectInput{Bucket: &c.bucket, Key: &key},
		awss3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.api.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		var nsk *s3types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		return err
	}
	return nil
}
