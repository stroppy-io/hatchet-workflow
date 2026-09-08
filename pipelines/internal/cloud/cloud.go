// Package cloud talks to the providers' own APIs for the two service
// pipelines: verifying a tenant's credentials and reading its quotas. It
// runs in ACTIVITY bodies on the run worker; the credentials are resolved
// from the Graphene secret at the point of use and never leave the process.
package cloud

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/graphene-ci/pipeline/pkg/workerapi"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Client verifies credentials and reads quotas of one provider.
type Client interface {
	Verify(ctx context.Context, settings json.RawMessage, credentials string, dryRun bool) (spec.ProviderVerifyResult, error)
	Quotas(ctx context.Context, settings json.RawMessage, credentials, location string) (spec.QuotasResult, error)
}

// For picks the client of a provider.
func For(kind spec.ProviderKind) (Client, error) {
	switch kind {
	case spec.ProviderYandex:
		return Yandex{}, nil
	case spec.ProviderAWS:
		return AWS{}, nil
	default:
		return nil, fmt.Errorf("cloud: unsupported provider %q", kind)
	}
}

// Credentials resolves the provider credentials secret to its JSON value.
func Credentials(ctx context.Context, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("cloud: credentials secret name is empty")
	}
	return workerapi.GetSecret(ctx, secret)
}

// permission records one checked right.
func permission(name string, err error) spec.Permission {
	return spec.Permission{Name: name, Granted: err == nil}
}
