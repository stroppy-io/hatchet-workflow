package repositories

import (
	"context"
	"errors"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/jackc/pgx/v5"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/pipelines"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/gen/db"
)

// PipelineRepo stores the per-namespace push state.
type PipelineRepo struct{ q *db.Queries }

// NewPipelineRepo builds the repository.
func NewPipelineRepo(database tx.DB) *PipelineRepo { return &PipelineRepo{q: db.New(database)} }

func (r *PipelineRepo) All(ctx context.Context) ([]pipelines.State, error) {
	rows, err := r.q.PipelinePushes(ctx)
	if err != nil {
		return nil, infraf("pipelines: all: %v", err)
	}
	out := make([]pipelines.State, 0, len(rows))
	for _, row := range rows {
		out = append(out, pipelines.State{Namespace: row.Namespace, Revision: row.Revision, Status: row.Status, Error: row.Error, PushedAt: row.PushedAt, UpdatedAt: row.UpdatedAt})
	}
	return out, nil
}

func (r *PipelineRepo) Get(ctx context.Context, namespace string) (pipelines.State, bool, error) {
	row, err := r.q.PipelinePush(ctx, namespace)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pipelines.State{}, false, nil
		}
		return pipelines.State{}, false, infraf("pipelines: get: %v", err)
	}
	return pipelines.State{Namespace: row.Namespace, Revision: row.Revision, Status: row.Status, Error: row.Error, PushedAt: row.PushedAt, UpdatedAt: row.UpdatedAt}, true, nil
}

func (r *PipelineRepo) Set(ctx context.Context, s pipelines.State) error {
	if err := r.q.SetPipelinePush(ctx, db.SetPipelinePushParams{Namespace: s.Namespace, Revision: s.Revision, Status: s.Status, Error: s.Error, PushedAt: s.PushedAt}); err != nil {
		return infraf("pipelines: set: %v", err)
	}
	return nil
}

func (r *PipelineRepo) Delete(ctx context.Context, namespace string) error {
	if err := r.q.DeletePipelinePush(ctx, namespace); err != nil {
		return infraf("pipelines: delete: %v", err)
	}
	return nil
}
