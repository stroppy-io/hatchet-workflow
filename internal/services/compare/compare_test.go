package compare

// compare_test.go: unit tests for CompareService.CompareRuns (compare.go).
//
// Coverage targets:
//   - Success: two runs in the same tenant, metrics returned
//   - Unauthenticated: Authn.Caller returns an error
//   - InvalidArgument: empty tenant_id
//   - InvalidArgument: fewer than 2 run_ids
//   - InvalidArgument: more than 16 run_ids
//   - InvalidArgument: empty run_id string in the list
//   - InvalidArgument: duplicate run_ids
//   - NotFound (run): Runs.Get returns derrors.ErrNotFound
//   - NotFound (foreign tenant): run belongs to a different tenant
//   - Internal: Runs.Get returns a non-domain error
//   - NotFound (metrics): Metrics.Compare returns derrors.ErrNotFound
//   - Internal: Metrics.Compare returns a non-domain error

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	apipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	modelspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	monitorpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// makeRecord constructs a minimal TestRunRecord belonging to tenantID.
func makeRecord(runID, tenantID string) *modelspb.TestRunRecord {
	return &modelspb.TestRunRecord{
		Entity: &commonpb.Entity{
			Id:       runID,
			TenantId: tenantID,
		},
	}
}

// makeService wires up a CompareService with all mocks injected.
func makeService(
	authn *utils.MockAuthn,
	runs *MockTestRunReader,
	metrics *MockMetricsComparator,
) *CompareService {
	return NewCompareService(CompareDeps{
		Authn:   authn,
		Runs:    runs,
		Metrics: metrics,
		Tx:      &utils.MockTrm{},
	})
}

func TestCompareRuns_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	const tenantID = "tenant-1"

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc-1"}, nil)
	runs.EXPECT().Get(ctx, "run-1").Return(makeRecord("run-1", tenantID), nil)
	runs.EXPECT().Get(ctx, "run-2").Return(makeRecord("run-2", tenantID), nil)
	metrics.EXPECT().Compare(ctx, []string{"run-1", "run-2"}).Return(&monitorpb.Comparison{}, nil)

	resp, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: tenantID,
		RunIds:   []string{"run-1", "run-2"},
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp.GetView() == nil {
		t.Error("expected non-nil view")
	}
	if len(resp.GetView().GetColumns()) != 2 {
		t.Errorf("expected 2 columns, got %d", len(resp.GetView().GetColumns()))
	}
}

func TestCompareRuns_ThreeRuns_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	const tenantID = "tenant-x"

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "acc-1"}, nil)
	runs.EXPECT().Get(ctx, "r1").Return(makeRecord("r1", tenantID), nil)
	runs.EXPECT().Get(ctx, "r2").Return(makeRecord("r2", tenantID), nil)
	runs.EXPECT().Get(ctx, "r3").Return(makeRecord("r3", tenantID), nil)
	metrics.EXPECT().Compare(ctx, []string{"r1", "r2", "r3"}).Return(&monitorpb.Comparison{}, nil)

	resp, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: tenantID,
		RunIds:   []string{"r1", "r2", "r3"},
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(resp.GetView().GetColumns()) != 3 {
		t.Errorf("expected 3 columns, got %d", len(resp.GetView().GetColumns()))
	}
}

func TestCompareRuns_CallerError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(nil, errors.New("no auth"))

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"r1", "r2"},
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestCompareRuns_EmptyTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := makeService(authn, NewMockTestRunReader(ctrl), NewMockMetricsComparator(ctrl))

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "",
		RunIds:   []string{"r1", "r2"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty tenant_id, got %v", err)
	}
}

func TestCompareRuns_TooFewRuns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := makeService(authn, NewMockTestRunReader(ctrl), NewMockMetricsComparator(ctrl))

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"r1"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for < 2 runs, got %v", err)
	}
}

func TestCompareRuns_ZeroRuns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := makeService(authn, NewMockTestRunReader(ctrl), NewMockMetricsComparator(ctrl))

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   nil,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for 0 runs, got %v", err)
	}
}

func TestCompareRuns_TooManyRuns(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := makeService(authn, NewMockTestRunReader(ctrl), NewMockMetricsComparator(ctrl))

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)

	ids := make([]string, 17)
	for i := range ids {
		ids[i] = "r" + string(rune('a'+i))
	}

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   ids,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for > 16 runs, got %v", err)
	}
}

func TestCompareRuns_EmptyRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := makeService(authn, NewMockTestRunReader(ctrl), NewMockMetricsComparator(ctrl))

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"r1", ""},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for empty run id, got %v", err)
	}
}

func TestCompareRuns_DuplicateRunIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	svc := makeService(authn, NewMockTestRunReader(ctrl), NewMockMetricsComparator(ctrl))

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"r1", "r1"},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for duplicate run ids, got %v", err)
	}
}

func TestCompareRuns_RunNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)
	runs.EXPECT().Get(ctx, "r1").Return(nil, derrors.ErrNotFound)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"r1", "r2"},
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
}

func TestCompareRuns_RunGetInternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)
	runs.EXPECT().Get(ctx, "r1").Return(nil, errors.New("db connection failed"))

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "t1",
		RunIds:   []string{"r1", "r2"},
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal for non-domain error, got %v", err)
	}
}

func TestCompareRuns_ForeignTenantRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)
	// r1 belongs to tenant-1 (the request's tenant)
	runs.EXPECT().Get(ctx, "r1").Return(makeRecord("r1", "tenant-1"), nil)
	// r2 belongs to a different tenant (tenant-other)
	runs.EXPECT().Get(ctx, "r2").Return(makeRecord("r2", "tenant-other"), nil)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: "tenant-1",
		RunIds:   []string{"r1", "r2"},
	})
	// assertTenant returns NotFound to avoid leaking cross-tenant run existence
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound for foreign-tenant run, got %v", err)
	}
}

func TestCompareRuns_MetricsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	const tenantID = "tenant-1"

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)
	runs.EXPECT().Get(ctx, "r1").Return(makeRecord("r1", tenantID), nil)
	runs.EXPECT().Get(ctx, "r2").Return(makeRecord("r2", tenantID), nil)
	metrics.EXPECT().Compare(ctx, []string{"r1", "r2"}).Return(nil, derrors.ErrNotFound)

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: tenantID,
		RunIds:   []string{"r1", "r2"},
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound when metrics not found, got %v", err)
	}
}

func TestCompareRuns_MetricsInternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	const tenantID = "tenant-1"

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)
	runs.EXPECT().Get(ctx, "r1").Return(makeRecord("r1", tenantID), nil)
	runs.EXPECT().Get(ctx, "r2").Return(makeRecord("r2", tenantID), nil)
	metrics.EXPECT().Compare(ctx, []string{"r1", "r2"}).Return(nil, errors.New("metrics db error"))

	_, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: tenantID,
		RunIds:   []string{"r1", "r2"},
	})
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal for metrics non-domain error, got %v", err)
	}
}

func TestCompareRuns_BaselineIsFirstRun(t *testing.T) {
	// Verifies that the columns align 1:1 with run_ids and columns[0] is
	// the baseline (first run).
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	authn := utils.NewMockAuthn(ctrl)
	runs := NewMockTestRunReader(ctrl)
	metrics := NewMockMetricsComparator(ctrl)
	svc := makeService(authn, runs, metrics)

	ctx := context.Background()
	const tenantID = "tenant-1"

	authn.EXPECT().Caller(ctx).Return(&iampb.AccessClaims{AccountId: "a"}, nil)
	runs.EXPECT().Get(ctx, "baseline").Return(makeRecord("baseline", tenantID), nil)
	runs.EXPECT().Get(ctx, "candidate").Return(makeRecord("candidate", tenantID), nil)
	metrics.EXPECT().Compare(ctx, []string{"baseline", "candidate"}).Return(&monitorpb.Comparison{}, nil)

	resp, err := svc.CompareRuns(ctx, &apipb.CompareRunsRequest{
		TenantId: tenantID,
		RunIds:   []string{"baseline", "candidate"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cols := resp.GetView().GetColumns()
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
	if cols[0].GetRunId() != "baseline" {
		t.Errorf("expected baseline first, got %s", cols[0].GetRunId())
	}
	if cols[1].GetRunId() != "candidate" {
		t.Errorf("expected candidate second, got %s", cols[1].GetRunId())
	}
}
