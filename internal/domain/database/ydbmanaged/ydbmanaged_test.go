package ydbmanaged

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestBuildTopologySpecUsesManagedGRPCSPort(t *testing.T) {
	spec, err := (&Database{}).BuildTopologySpec(&domain.YdbManagedParams{
		Type: domain.YdbManagedParams_TYPE_SERVERLESS,
	})
	if err != nil {
		t.Fatalf("build topology spec: %v", err)
	}

	if got, want := spec.GetLabels()["managed_grpc_port"], "2135"; got != want {
		t.Fatalf("managed_grpc_port = %q, want %q", got, want)
	}
}
