package compare

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	The service owns no storage or compute of its own: every side effect goes
	through one of these. Implementations live elsewhere and are wired in via
	CompareDeps. Persistence methods return derrors.ErrNotFound / derrors.ErrConflict
	so the handler can translate them into status codes via utils.MapErr.
*/

// TestRunReader loads stored run records for the comparison. Get returns
// derrors.ErrNotFound for an unknown id; the handler additionally enforces that
// the record belongs to the request's tenant so a caller cannot pull a foreign
// tenant's run into a comparison.
type TestRunReader interface {
	Get(ctx context.Context, id string) (*models.Run, error)
}

// MetricsComparator computes the per-metric, cross-run diff (monitor.Comparison)
// for the given runs in display order; runIDs[0] is the baseline. It reads the
// runs' persisted metric samples and returns the aligned MetricRow/RunSummary
// matrix the compare page renders. Implementations report derrors.ErrNotFound
// when a run has no metrics to compare.
type MetricsComparator interface {
	Compare(ctx context.Context, runIDs []string) (*monitor.Comparison, error)
}

// CompareDeps bundles every dependency for the constructor.
type CompareDeps struct {
	Authn   utils.Authn
	Runs    TestRunReader
	Metrics MetricsComparator
	Tx      tx.Trm
}

type CompareService struct {
	*api.UnimplementedCompareServiceServer
	tx.Trm
	d CompareDeps
}

var _ api.CompareServiceServer = (*CompareService)(nil)

func NewCompareService(deps CompareDeps) *CompareService {
	return &CompareService{UnimplementedCompareServiceServer: &api.UnimplementedCompareServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *CompareService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *CompareService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *CompareService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *CompareService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// runColumn projects a stored run record into the comparison's config column.
func runColumn(rec *models.Run) *api.RunColumn {
	sum := rec.GetSummary()
	return &api.RunColumn{
		RunId:          rec.GetEntity().GetId(),
		Name:           rec.GetEntity().GetName(),
		Status:         rec.GetStatus(),
		DbKind:         sum.GetDbKind(),
		DbName:         sum.GetDbPresetName(),
		WorkloadName:   sum.GetWorkloadName(),
		StroppyVersion: sum.GetStroppyVersion(),
		Provider:       sum.GetProvider(),
		TopologyLabel:  sum.GetTopologyLabel(),
		NodeCount:      sum.GetNodeCount(),
		StartedAt:      sum.GetStartedAt(),
		Duration:       sum.GetDuration(),
	}
}
