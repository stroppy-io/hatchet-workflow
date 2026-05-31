// Package gateway is the single agent-facing entrypoint of the stroppy-cloud
// server. Agents are provisioned knowing ONLY the server address; everything
// they need is reached through it:
//
//   - Temporal: a transparent gRPC proxy forwards every unknown gRPC service
//     (i.e. the Temporal frontend services the agent's worker speaks) to the
//     real Temporal frontend. The agent dials the gateway as if it were
//     Temporal itself.
//   - The agent binary: GET /agent/binary streams the linux agent executable so
//     a bare base image can bootstrap itself (cloud-init style).
//   - Artifacts (stroppy, exporters, ...): GET /artifacts/{name} and
//     GET /api/binaries/{name}/{ver}/{file} proxy + cache upstream downloads so
//     agents never reach the internet directly.
//   - apt: the gateway doubles as a forward HTTP proxy (absolute-URI GET +
//     CONNECT) so `apt-get` inside the agent goes through the server and its
//     package cache.
//
// gRPC (Temporal proxy) and HTTP (everything else) share ONE listener via cmux,
// so the agent truly needs a single host:port.
package gateway

import (
	"context"

	"github.com/siderolabs/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// newTemporalProxy builds a gRPC server that transparently forwards every RPC to
// the Temporal frontend at temporalHostPort. It registers no services of its
// own: a raw passthrough codec + UnknownServiceHandler proxies the bytes through
// untouched, so the agent's Temporal worker (polls, sessions, signals, queries)
// works exactly as if connected directly.
func newTemporalProxy(temporalHostPort string) (*grpc.Server, *grpc.ClientConn, error) {
	backend, err := grpc.NewClient(
		temporalHostPort,
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(proxy.Codec())),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	director := func(ctx context.Context, _ string) (proxy.Mode, []proxy.Backend, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		return proxy.One2One, []proxy.Backend{
			&proxy.SingleBackend{
				GetConn: func(context.Context) (context.Context, *grpc.ClientConn, error) {
					return outCtx, backend, nil
				},
			},
		}, nil
	}

	srv := grpc.NewServer(
		grpc.ForceServerCodecV2(proxy.Codec()),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
	)
	return srv, backend, nil
}
