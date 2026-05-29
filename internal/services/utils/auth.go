package utils

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// Authn resolves the caller's verified access claims from the request context
// (populated upstream by the auth middleware). It is the one identity port
// every service depends on, so it lives here instead of being re-declared in
// each package.
type Authn interface {
	Caller(ctx context.Context) (*iam.AccessClaims, error)
}
