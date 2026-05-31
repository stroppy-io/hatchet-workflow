package api

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// RunStore persists run records + comparison baselines. Backed by the existing
// `runs` + `baselines` tables (the `runs.snapshot` column now stores a
// types.RunRecord JSON instead of a dag.Snapshot). Status is NOT stored here —
// it is queried live from Temporal (App.Status).
type RunStore interface {
	// Create upserts a run record (called when a run is launched).
	Create(ctx context.Context, rec types.RunRecord) error
	// Get loads a run record by id, or returns nil when not found.
	Get(ctx context.Context, tenantID, id string) (*types.RunRecord, error)
	// List returns all run records for a tenant, newest first.
	List(ctx context.Context, tenantID string) ([]types.RunRecord, error)
	// Delete removes a run record.
	Delete(ctx context.Context, tenantID, id string) error

	// SetBaseline names a run as the comparison baseline.
	SetBaseline(ctx context.Context, tenantID, name, runID string) error
	// GetBaseline resolves a baseline name to its run id.
	GetBaseline(ctx context.Context, tenantID, name string) (string, error)
	// ListBaselines returns every named baseline for the tenant.
	ListBaselines(ctx context.Context, tenantID string) (map[string]string, error)
}
