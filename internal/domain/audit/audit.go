// Package audit records who did what. Entries are written inside the
// mutation's transaction when there is one; reading is per tenant.
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
)

// ActorKind of an entry.
type ActorKind string

// Actor kinds.
const (
	ActorUser   ActorKind = "user"
	ActorToken  ActorKind = "token"
	ActorSystem ActorKind = "system"
	ActorAdmin  ActorKind = "admin"
)

// Target of an action.
type Target struct {
	Kind string
	ID   string
	Name string
}

// Entry is one audit record.
type Entry struct {
	ID        int64
	At        time.Time
	TenantID  *uuid.UUID
	ActorKind ActorKind
	ActorID   string
	ActorName string
	Action    string
	Target    Target
	Details   map[string]any
	RequestID string
}

// Query is the tenant slice filter.
type Query struct {
	TenantID uuid.UUID
	BeforeID int64
	Action   string
	ActorID  string
	Since    *time.Time
	Limit    int
}

// Repository is the storage port.
type Repository interface {
	Insert(ctx context.Context, e Entry) error
	OfTenant(ctx context.Context, q Query) ([]Entry, error)
}

// RequestID reads the request id of ctx; wired by the transport.
type RequestID func(ctx context.Context) string

// Service writes and reads the log.
type Service struct {
	repo      Repository
	requestID RequestID
}

// NewService builds the service.
func NewService(repo Repository, requestID RequestID) *Service {
	return &Service{repo: repo, requestID: requestID}
}

// Record fills the actor from ctx and writes the entry.
func (s *Service) Record(ctx context.Context, e Entry) error {
	a := auth.ActorFrom(ctx)
	switch {
	case e.ActorKind == ActorAdmin && !a.IsAnonymous():
		e.ActorID, e.ActorName = a.UserID.String(), a.Email
	case a.IsAPIToken():
		e.ActorKind, e.ActorID = ActorToken, a.TokenID.String()
	case !a.IsAnonymous():
		e.ActorKind, e.ActorID, e.ActorName = ActorUser, a.UserID.String(), a.Email
	default:
		e.ActorKind = ActorSystem
	}
	if s.requestID != nil {
		e.RequestID = s.requestID(ctx)
	}
	return s.repo.Insert(ctx, e)
}

// OfTenant reads a page (limit+1 rows: has_more is the caller's to derive).
func (s *Service) OfTenant(ctx context.Context, q Query) ([]Entry, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	return s.repo.OfTenant(ctx, q)
}
