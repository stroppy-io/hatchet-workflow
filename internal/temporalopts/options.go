package temporalopts

import (
	"crypto/tls"

	temporalclient "go.temporal.io/sdk/client"
	"google.golang.org/grpc"
)

const (
	// MaxMessageSizeBytes keeps self-hosted smoke runs away from gRPC's 4 MiB
	// default while the workflows are being trimmed to avoid oversized histories.
	MaxMessageSizeBytes = 512 * 1024 * 1024
)

func ConnectionOptions(tlsConfig *tls.Config) temporalclient.ConnectionOptions {
	return temporalclient.ConnectionOptions{
		TLS:            tlsConfig,
		MaxPayloadSize: MaxMessageSizeBytes,
		DialOptions: []grpc.DialOption{
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(MaxMessageSizeBytes),
				grpc.MaxCallSendMsgSize(MaxMessageSizeBytes),
			),
		},
	}
}
