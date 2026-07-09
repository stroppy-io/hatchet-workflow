package compare

import (
	"context"
	"testing"

	"github.com/avito-tech/go-transaction-manager/trm"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domainpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
)

// TestCompareRuns_ReadsRunSummaryNotTestRunRecord asserts CompareService reads
// runs through the models.Run-typed TestRunReader port (SP-E Task 5's
// compare/rating/dashboard/quota/favorite/share cutover) rather than the
// retired models.TestRunRecord shape.
func TestCompareRuns_ReadsRunSummaryNotTestRunRecord(t *testing.T) {
	svc := NewCompareService(CompareDeps{
		Authn: fakeAuthn{},
		Runs: fakeRunGetter{"run-1": {
			Entity:  &commonpb.Entity{Id: "run-1", TenantId: "t1"},
			Summary: &models.Run_Summary{DbKind: domainpb.Database_KIND_POSTGRES},
		}, "run-2": {
			Entity:  &commonpb.Entity{Id: "run-2", TenantId: "t1"},
			Summary: &models.Run_Summary{DbKind: domainpb.Database_KIND_POSTGRES},
		}},
		Metrics: fakeMetricsComparator{},
		Tx:      fakeTrm{},
	})

	resp, err := svc.CompareRuns(context.Background(), &api.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"run-1", "run-2"},
	})
	if err != nil {
		t.Fatalf("compare runs: %v", err)
	}
	if got := resp.GetView().GetColumns()[0].GetDbKind(); got != domainpb.Database_KIND_POSTGRES {
		t.Fatalf("db_kind = %v, want KIND_POSTGRESQL", got)
	}
}

type fakeAuthn struct{}

func (fakeAuthn) Caller(context.Context) (*iam.AccessClaims, error) {
	return &iam.AccessClaims{AccountId: "account-1"}, nil
}

type fakeRunGetter map[string]*models.Run

func (g fakeRunGetter) Get(_ context.Context, id string) (*models.Run, error) {
	rec, ok := g[id]
	if !ok {
		return nil, derrors.NotFound("run", "run not found")
	}
	return rec, nil
}

type fakeMetricsComparator struct{}

func (fakeMetricsComparator) Compare(_ context.Context, runIDs []string) (*monitor.Comparison, error) {
	return &monitor.Comparison{RunIds: runIDs}, nil
}

// fakeTrm is a no-op transaction manager: it runs fn directly without wrapping
// it in a real database transaction, since these unit tests exercise pure
// service logic over in-memory fakes.
type fakeTrm struct{}

func (fakeTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (fakeTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
