package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
)

// Uploader issues presigned PUT URLs for object uploads (packages.Uploader, G2):
// the client uploads the .deb bytes directly to object storage, not through RPC.
type Uploader struct {
	presign *s3sdk.PresignClient
	bucket  string
}

// NewUploader builds an Uploader over an S3 client + bucket.
func NewUploader(client *s3sdk.Client, bucket string) *Uploader {
	return &Uploader{presign: s3sdk.NewPresignClient(client), bucket: bucket}
}

// PresignPut returns a presigned PUT URL for objectKey valid for ttl.
func (u *Uploader) PresignPut(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	req, err := u.presign.PresignPutObject(ctx, &s3sdk.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(objectKey),
	}, s3sdk.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("s3: presign put %q: %w", objectKey, err)
	}
	return req.URL, nil
}
