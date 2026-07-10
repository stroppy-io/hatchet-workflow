package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/gateway"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// ShareTokenReader is the storage port used to resolve a public share token.
// It is the same repo the public_share service reads.
type ShareTokenReader interface {
	GetByToken(ctx context.Context, token string) (*models.ShareRecord, error)
}

// ShareScopeResolver implements gateway.ShareResolver: it turns a public share
// token into the single test run that token exposes, so the gateway can pin
// every share-scoped metrics query to that run.
//
// Only TEST_RUN shares grant a metrics scope — a suite-run share has no single
// stroppy_run_id to filter on. Unknown, revoked, expired and non-test-run
// tokens all return the same error, so a caller cannot tell them apart.
type ShareScopeResolver struct {
	shares ShareTokenReader
	now    func() time.Time
}

var _ gateway.ShareResolver = (*ShareScopeResolver)(nil)

// errNoShareScope is returned for every unusable token. It is deliberately
// uniform: the gateway maps it to a plain 404.
var errNoShareScope = errors.New("share: no scope for token")

func NewShareScopeResolver(shares ShareTokenReader) *ShareScopeResolver {
	return &ShareScopeResolver{shares: shares, now: time.Now}
}

func (r *ShareScopeResolver) ResolveShare(ctx context.Context, token string) (gateway.ShareScope, error) {
	if r == nil || r.shares == nil || token == "" {
		return gateway.ShareScope{}, errNoShareScope
	}
	rec, err := r.shares.GetByToken(ctx, token)
	if err != nil || rec == nil || !shareIsLive(rec, r.now()) {
		return gateway.ShareScope{}, errNoShareScope
	}
	target := rec.GetTarget()
	if target.GetKind() != models.ShareRecord_Target_KIND_TEST_RUN || target.GetId() == "" {
		return gateway.ShareScope{}, errNoShareScope
	}

	scope := gateway.ShareScope{RunID: target.GetId()}
	// The snapshot carries the run's own window; use it to clamp queries. A run
	// still in flight has no finished_at, so the window stays open at the top.
	if view := rec.GetSnapshot().GetTestRun(); view != nil {
		if ts := view.GetStartedAt(); ts != nil {
			scope.From = ts.AsTime()
		}
		if ts := view.GetFinishedAt(); ts != nil {
			scope.To = ts.AsTime()
		}
	}
	return scope, nil
}

// shareIsLive mirrors public_share.isLive: revoked or past expiry is dead. An
// unset expiry never expires.
func shareIsLive(rec *models.ShareRecord, now time.Time) bool {
	if rec.GetRevoked() {
		return false
	}
	if exp := rec.GetExpiresAt(); exp != nil && !now.Before(exp.AsTime()) {
		return false
	}
	return true
}
