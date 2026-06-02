package rating

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deployment "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Dependency interfaces (constructor-injected) =====

	rating computes benchmark leaderboards; it owns no storage. Ratings are NOT
	stored as DB records (see rating.proto): they are computed/precomputed from the
	pool of benchmark runs flagged for a board. The service therefore reads ranked
	rows through a RatingBoard port and shapes them into RatingEntry, leaving the
	heavy lifting (precompute, ranking SQL, pagination cursor) to the
	implementation. Persistence ports return derrors.ErrNotFound / derrors.ErrConflict
	so handlers can translate them via utils.MapErr.
*/

// Scope distinguishes which pool of runs a board ranks. SCOPE_SYSTEM ranks every
// run flagged in_global_rating across all tenants; SCOPE_TENANT ranks only the
// runs flagged in_tenant_rating that belong to one tenant.
type Scope int

const (
	ScopeSystem Scope = iota
	ScopeTenant
)

// RatingQuery is the normalized, validated request the board implementation
// consumes. It carries the selected scope (and, for SCOPE_TENANT, the tenant the
// handler has already verified), the ranking metric, the AND-ed narrowing facets,
// the page size and the opaque pagination cursor.
type RatingQuery struct {
	Scope           Scope
	TenantID        string // set iff Scope == ScopeTenant
	MetricKey       string
	DBKinds         []domain.Database_Kind
	StroppyVersions []string
	Providers       []deployment.Provider
	StartedAfter    *time.Time
	StartedBefore   *time.Time
	Limit           uint32
	PageToken       string
}

// RatingBoard ranks the benchmark runs for one board. Implementations rank by
// RatingQuery.MetricKey honoring that metric's own higher_is_better, apply every
// facet as an AND filter, page by the opaque token, and return entries already
// ordered with their absolute rank assigned. The system board sets RatingEntry
// tenant_name; the tenant board leaves it empty. Rank returns derrors.ErrNotFound
// when the metric_key is not a known MetricSummary key.
type RatingBoard interface {
	Rank(ctx context.Context, q RatingQuery) (entries []*api.RatingEntry, nextPageToken string, err error)
}

// RatingDeps bundles every dependency for the constructor.
type RatingDeps struct {
	Authn   utils.Authn
	Board   RatingBoard
	Tenants utils.TenantReader
	Tx      tx.Trm
}

type RatingService struct {
	*api.UnimplementedRatingServiceServer
	tx.Trm
	d RatingDeps
}

var _ api.RatingServiceServer = (*RatingService)(nil)

func NewRatingService(deps RatingDeps) *RatingService {
	return &RatingService{UnimplementedRatingServiceServer: &api.UnimplementedRatingServiceServer{}, d: deps}
}

/*
	===== helpers =====
*/

func (s *RatingService) caller(ctx context.Context) (*iam.AccessClaims, error) {
	c, err := s.d.Authn.Caller(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return c, nil
}

func (s *RatingService) now() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// doTx runs fn in a top-level serializable transaction, retrying on transient
// serialization failures/deadlocks (DefaultRetryPolicy). fn is re-run from
// scratch on retry, so it MUST be safe to repeat: DB-only side effects and never
// synchronous external IO.
func (s *RatingService) doTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.DoSerializable(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// doTxRet is doTx for a transaction that returns a value. Same retry semantics:
// fn must be safe to re-run.
func doTxRet[T any](ctx context.Context, s *RatingService, fn func(ctx context.Context) (T, error)) (T, error) {
	return tx.DoSerializableRet(ctx, s.d.Tx, fn, tx.WithRetry(tx.DefaultRetryPolicy))
}

// validateFilter enforces the documented invariants on RatingFilter: metric_key
// is required (the ranking dimension) and a started_after/started_before window
// must be ordered.
func validateFilter(f *api.RatingFilter) error {
	if f == nil {
		return status.Error(codes.InvalidArgument, "filter is required")
	}
	if f.GetMetricKey() == "" {
		return status.Error(codes.InvalidArgument, "filter.metric_key is required")
	}
	after, before := f.GetStartedAfter(), f.GetStartedBefore()
	if after != nil && before != nil && after.AsTime().After(before.AsTime()) {
		return status.Error(codes.InvalidArgument, "filter.started_after must not be after started_before")
	}
	return nil
}

// toQuery builds the normalized RatingQuery for a board from a validated filter.
func toQuery(scope Scope, tenantID string, f *api.RatingFilter, limit uint32, pageToken string) RatingQuery {
	q := RatingQuery{
		Scope:           scope,
		TenantID:        tenantID,
		MetricKey:       f.GetMetricKey(),
		DBKinds:         f.GetDbKinds(),
		StroppyVersions: f.GetStroppyVersions(),
		Providers:       f.GetProviders(),
		Limit:           limit,
		PageToken:       pageToken,
	}
	if t := f.GetStartedAfter(); t != nil {
		at := t.AsTime()
		q.StartedAfter = &at
	}
	if t := f.GetStartedBefore(); t != nil {
		bt := t.AsTime()
		q.StartedBefore = &bt
	}
	return q
}

/*
	===== Boards =====
*/

// GetSystemRating ranks the cross-system private board over the runs flagged
// in_global_rating. The interceptor only requires an authenticated caller (no
// specific permission), so we resolve identity to confirm a verified caller is
// present and then rank the global pool.
func (s *RatingService) GetSystemRating(ctx context.Context, req *api.GetSystemRatingRequest) (*api.GetSystemRatingResponse, error) {
	if _, err := s.caller(ctx); err != nil {
		return nil, err
	}
	if err := validateFilter(req.GetFilter()); err != nil {
		return nil, err
	}
	q := toQuery(ScopeSystem, "", req.GetFilter(), req.GetLimit(), req.GetPageToken())
	entries, next, err := s.d.Board.Rank(ctx, q)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetSystemRatingResponse{Entries: entries, NextPageToken: next}, nil
}

// GetTenantRating ranks one tenant's board over the runs flagged in_tenant_rating.
// The RESOURCE_TEST_RUN/LIST permission is enforced by the interceptor; here we
// validate the active tenant_id and verify it exists before ranking, so a stale
// or foreign id surfaces as NotFound rather than an empty board.
func (s *RatingService) GetTenantRating(ctx context.Context, req *api.GetTenantRatingRequest) (*api.GetTenantRatingResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := validateFilter(req.GetFilter()); err != nil {
		return nil, err
	}
	if _, err := s.d.Tenants.Get(ctx, req.GetTenantId()); err != nil {
		return nil, utils.MapErr(err)
	}
	q := toQuery(ScopeTenant, req.GetTenantId(), req.GetFilter(), req.GetLimit(), req.GetPageToken())
	entries, next, err := s.d.Board.Rank(ctx, q)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetTenantRatingResponse{Entries: entries, NextPageToken: next}, nil
}
