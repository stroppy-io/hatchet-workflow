// Package auth is who is calling: the actor resolved from a verified token
// and its context plumbing. It knows nothing about tenants or roles — that
// is authorization, decided per operation by the services.
package auth

import (
	"context"

	"github.com/google/uuid"
)

// Actor is the verified caller of one request. Zero value = anonymous.
type Actor struct {
	// UserID is the IAM subject (profile id).
	UserID uuid.UUID
	// SessionID is the IAM session (empty for API tokens).
	SessionID string
	// Email as IAM reported it in the token or profile; may be empty.
	Email string
	// TokenID is set when the actor came through a personal API token; such
	// an actor is confined to TokenTenant with TokenRole.
	TokenID     uuid.UUID
	TokenTenant uuid.UUID
	TokenRole   string
}

// IsAnonymous reports the zero actor (no person and no token).
func (a Actor) IsAnonymous() bool { return a.UserID == uuid.Nil && a.TokenID == uuid.Nil }

// IsAPIToken reports an actor confined by a personal API token.
func (a Actor) IsAPIToken() bool { return a.TokenID != uuid.Nil }

type actorKey struct{}

// WithActor stores the actor in ctx.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the actor of ctx (anonymous when none).
func ActorFrom(ctx context.Context) Actor {
	a, _ := ctx.Value(actorKey{}).(Actor) //nolint:errcheck // absent = anonymous
	return a
}
