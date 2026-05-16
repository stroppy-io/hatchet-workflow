package testing

import (
	"context"

	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// QuotaPort enforces per-tenant resource quotas.
type QuotaPort interface {
	CheckAndReserve(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error
	Release(ctx context.Context, tenantID *iampb.TenantId, resourceID string, amount int64) error
}

// quotaNoop is a no-op QuotaPort for tests that don't need quota enforcement.
type quotaNoop struct{}

func (quotaNoop) CheckAndReserve(_ context.Context, _ *iampb.TenantId, _ string, _ int64) error {
	return nil
}
func (quotaNoop) Release(_ context.Context, _ *iampb.TenantId, _ string, _ int64) error {
	return nil
}

// NoopQuota returns a QuotaPort that always succeeds.
func NoopQuota() QuotaPort { return quotaNoop{} }

type IamPort interface {
	HasTenantRole(ctx context.Context, u *iampb.UserId, t *iampb.TenantId, min iampb.TenantRole) (bool, error)
	UserFromCtx(ctx context.Context) (*iampb.UserId, error)
	TenantFromCtx(ctx context.Context) (*iampb.TenantId, error)
}

type CatalogPort interface {
	GetDatabasePreset(ctx context.Context, id *catalogpb.DatabasePresetId) (*catalogpb.DatabasePreset, error)
	GetWorkloadPreset(ctx context.Context, id *catalogpb.WorkloadPresetId) (*catalogpb.WorkloadPreset, error)
	GetPackage(ctx context.Context, id *catalogpb.PackageId) (*catalogpb.Package, error)
}

type SystemEnginePort interface {
	LaunchDag(ctx context.Context, template *systempb.Dag, metadata map[string]string) (*systempb.DagRun, error)
	CancelDagRun(ctx context.Context, id *systempb.DagRunId) error
	GetDagRun(ctx context.Context, id *systempb.DagRunId) (*systempb.DagRun, error)
	SubscribeProgress(ctx context.Context, dagRunID *systempb.DagRunId, since string) (<-chan *systempb.NodeRun, func(), error)
	StreamLogs(ctx context.Context, dagRunID *systempb.DagRunId, stepID string, since string) (<-chan *agentpb.LogLine, func(), error)
}

type MetricsPort interface {
	Query(ctx context.Context, tenant *iampb.TenantId, testRunID *testingpb.TestRunId, query string) (*testingpb.MetricSeriesList, error)
}

// DagBuilderPort converts a TestRun into a Dag template. Defined as an
// interface here to avoid an import cycle between the testing and dagbuilder
// packages.
type DagBuilderPort interface {
	FromTestRun(ctx context.Context, tr *testingpb.TestRun) (*systempb.Dag, error)
}
