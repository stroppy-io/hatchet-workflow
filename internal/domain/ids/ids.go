// Package ids generates entity identifiers (ULID, 26-char Crockford base32 — the
// shape models.Ulid validates with len=26).
package ids

import (
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// New returns a new ULID string (48-bit ms timestamp + 80-bit monotonic entropy).
func New() string {
	return ulid.Make().String()
}

// NewUlid returns a fresh models.Ulid.
func NewUlid() *models.Ulid {
	return &models.Ulid{Value: New()}
}

// NewEntity returns a fresh models.Entity with a new id and created/updated
// timestamps set to now — use when persisting a new aggregate.
func NewEntity() *models.Entity {
	now := timestamppb.Now()
	return &models.Entity{
		Id:         NewUlid(),
		Timestamps: &models.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}
