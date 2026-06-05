package compare

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

// CompareRuns assembles the side-by-side comparison page for N runs (2..16) in
// the request's display order; run_ids[0] is the baseline. It is read-only
// (NO_SIDE_EFFECTS): the RESOURCE_TEST_RUN/READ check is enforced by the auth
// interceptor, so here we only resolve the caller, validate the request, and
// confirm every run exists AND belongs to the active tenant before pulling it
// into the comparison — a caller must not be able to baseline against a foreign
// tenant's run. The per-run config columns align 1:1 with run_ids and the
// cross-run metric diff is computed over the same ordered set.
func (s *CompareService) CompareRuns(ctx context.Context, req *api.CompareRunsRequest) (*api.CompareRunsResponse, error) {
	if _, err := s.caller(ctx); err != nil {
		return nil, err
	}
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id required")
	}
	runIDs := req.GetRunIds()
	if len(runIDs) < 2 {
		return nil, status.Error(codes.InvalidArgument, "at least two runs are required to compare")
	}
	if len(runIDs) > 16 {
		return nil, status.Error(codes.InvalidArgument, "at most 16 runs can be compared")
	}
	seen := make(map[string]struct{}, len(runIDs))
	for _, id := range runIDs {
		if id == "" {
			return nil, status.Error(codes.InvalidArgument, "run id must not be empty")
		}
		if _, dup := seen[id]; dup {
			return nil, status.Error(codes.InvalidArgument, "run ids must be distinct")
		}
		seen[id] = struct{}{}
	}

	columns, err := doTxRet(ctx, s, func(ctx context.Context) ([]*api.RunColumn, error) {
		columns := make([]*api.RunColumn, 0, len(runIDs))
		for _, id := range runIDs {
			rec, err := s.d.Runs.Get(ctx, id)
			if err != nil {
				return nil, utils.MapErr(err)
			}
			if err := s.assertTenant(rec, req.GetTenantId()); err != nil {
				return nil, err
			}
			columns = append(columns, runColumn(rec))
		}
		return columns, nil
	})
	if err != nil {
		return nil, err
	}
	metrics, err := s.d.Metrics.Compare(ctx, runIDs)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	view := &api.CompareView{Columns: columns, Metrics: metrics}
	return &api.CompareRunsResponse{View: view}, nil
}

// assertTenant rejects a run that does not belong to the comparison's tenant.
// Returning NotFound (not PermissionDenied) avoids leaking the existence of a
// run in another tenant to the caller.
func (s *CompareService) assertTenant(rec *models.TestRunRecord, tenantID string) error {
	if rec.GetEntity().GetTenantId() != tenantID {
		return status.Errorf(codes.NotFound, "run %q not found in tenant", rec.GetEntity().GetId())
	}
	return nil
}
