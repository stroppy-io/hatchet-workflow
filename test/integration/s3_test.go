//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/docker/go-connections/nat"
	"github.com/gopherex/xlog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/s3"
)

// TestS3UploaderPresignPutRoundTrip exercises the REAL s3 infrastructure
// (s3.NewClient / s3.EnsureBucket / s3.NewUploader.PresignPut) against a real
// MinIO server. It proves the presigned PUT URL minted by the SDK actually
// round-trips: an external HTTP PUT to the presigned URL stores the bytes, and
// the same bytes read back through the s3 client are byte-identical.
func TestS3UploaderPresignPutRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const (
		rootUser = "minioadmin"
		rootPass = "minioadmin"
		bucket   = "stroppy-packages"
	)

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			Cmd:          []string{"server", "/data"},
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     rootUser,
				"MINIO_ROOT_PASSWORD": rootPass,
			},
			WaitingFor: wait.ForHTTP("/minio/health/live").
				WithPort("9000/tcp").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err, "minio must start")
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := container.MappedPort(ctx, nat.Port("9000/tcp"))
	require.NoError(t, err)
	endpoint := fmt.Sprintf("%s:%s", host, mappedPort.Port())

	// Build the s3 Config the same way production would, pointing at MinIO.
	// UseSSL=false -> http scheme; NewClient enables UsePathStyle internally,
	// which is what MinIO needs.
	cfg := &s3.Config{
		Endpoint:  endpoint,
		Region:    "us-east-1",
		AccessKey: rootUser,
		SecretKey: rootPass,
		Bucket:    bucket,
		UseSSL:    false,
	}
	logger := xlog.NewConsole()

	client, probe := s3.NewClient(cfg, logger)
	require.NotNil(t, client)
	require.NotNil(t, probe)

	require.NoError(t, s3.EnsureBucket(ctx, client, bucket), "EnsureBucket must create the bucket")

	// Bucket now exists -> the readiness probe (HeadBucket) must report Up.
	require.True(t, probe.Check(ctx).OK(), "s3 probe must report Up once bucket exists")

	// PresignPut -> a presigned URL that an arbitrary HTTP client can PUT to.
	uploader := s3.NewUploader(client, bucket)
	const objectKey = "uploads/example-1.0.0_amd64.deb"
	payload := []byte("fake .deb package bytes \x00\x01\x02 round-trip check")

	url, err := uploader.PresignPut(ctx, objectKey, 5*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, url)

	// Upload via the presigned URL using a plain net/http client (no AWS creds).
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	require.NoError(t, err)
	req.ContentLength = int64(len(payload))

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "presigned PUT must reach minio")
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equalf(t, http.StatusOK, resp.StatusCode,
		"presigned PUT must return 200, got %d: %s", resp.StatusCode, string(body))

	// HeadObject: length matches.
	head, err := client.HeadObject(ctx, &s3sdk.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	require.NoError(t, err, "HeadObject must find the presigned-uploaded object")
	require.Equal(t, int64(len(payload)), aws.ToInt64(head.ContentLength))

	// GetObject: bytes are identical.
	got, err := client.GetObject(ctx, &s3sdk.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	require.NoError(t, err)
	gotBytes, err := io.ReadAll(got.Body)
	_ = got.Body.Close()
	require.NoError(t, err)
	require.Equal(t, payload, gotBytes, "object bytes must survive the presigned PUT round-trip")
}
