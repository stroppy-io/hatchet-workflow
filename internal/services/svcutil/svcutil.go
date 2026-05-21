// Package svcutil holds tiny helpers shared by the service implementations:
// caller extraction and pgx-error-to-gRPC-status mapping.
package svcutil

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
)

// CallerOf returns the request principal (nil when anonymous; authz rejects nil).
func CallerOf(ctx context.Context) *caller.Caller {
	c, _ := caller.FromContext(ctx)
	return c
}

// NotFound maps pgx.ErrNoRows to codes.NotFound("<entity> not found") and any
// other error to codes.Internal.
func NotFound(err error, entity string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Errorf(codes.NotFound, "%s not found", entity)
	}
	return status.Errorf(codes.Internal, "load %s: %v", entity, err)
}
