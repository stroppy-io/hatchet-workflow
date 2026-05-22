// Package dagstore is the ratel-backed runtime.Storage: it maps the runtime's
// primitive.Dag onto the persisted models.Dag row (payload = serialized
// primitive.Dag, plus mirror columns status/lease). The DagProcessor polls it;
// RunService persists new runs through it. Tenant scoping travels in
// dag.Metadata["tenant_id"] (the runtime primitive is domain-agnostic).
package dagstore

import (
	"context"
	"errors"
	"time"

	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/runtime"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// Store satisfies the runtime processor's Storage contract.
var _ runtime.Storage = (*Store)(nil)

// MetadataTenantID is the dag.Metadata key carrying the owning tenant id.
const MetadataTenantID = "tenant_id"

// Store persists primitive.Dag aggregates. It implements runtime.Storage
// (ListDagsByStatus, SaveDag) and adds GetDag for the control plane.
type Store struct {
	*tracing.Entity
	dags *repository.ProtoRepository[
		models.DagAlias,
		models.DagColumnAlias,
		*models.DagScanner,
		*models.Dag,
	]
}

// New builds a Store over the given DB executor.
func New(logger *xlog.Logger, executor exec.DB) *Store {
	return &Store{
		Entity: tracing.NewEntity(logger.AppendName("DagStore")),
		dags: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Dags.Table, executor),
			models.DagConverter,
		),
	}
}

// ListDagsByStatus returns the payloads of dags currently in any of the statuses.
func (s *Store) ListDagsByStatus(ctx context.Context, statuses []primitive.Status) ([]*primitive.Dag, error) {
	statusNames := statusStrings(statuses)
	if len(statusNames) == 0 {
		return nil, nil
	}

	rows, err := s.dags.Query(ctx,
		models.Dags.SelectAll().Where(
			models.Dags.Status.In(statusNames...),
			models.Dags.DeletedAt.IsNull(),
		))
	if err != nil {
		return nil, err
	}

	var out []*primitive.Dag
	for _, row := range rows {
		out = append(out, row.GetPayload())
	}
	return out, nil
}

// ListByTenant returns the payloads of the tenant's dags in any of the statuses.
func (s *Store) ListByTenant(ctx context.Context, tenantID string, statuses []primitive.Status) ([]*primitive.Dag, error) {
	statusNames := statusStrings(statuses)
	if len(statusNames) == 0 {
		return nil, nil
	}

	rows, err := s.dags.Query(ctx,
		models.Dags.SelectAll().Where(
			models.Dags.TenantId.Eq(tenantID),
			models.Dags.Status.In(statusNames...),
			models.Dags.DeletedAt.IsNull(),
		))
	if err != nil {
		return nil, err
	}

	var out []*primitive.Dag
	for _, row := range rows {
		out = append(out, row.GetPayload())
	}
	return out, nil
}

func statusStrings(statuses []primitive.Status) []string {
	names := make([]string, 0, len(statuses))
	for _, st := range statuses {
		names = append(names, st.String())
	}
	return names
}

// GetDag loads a single dag payload by id. Returns (nil, nil) when absent.
func (s *Store) GetDag(ctx context.Context, id string) (*primitive.Dag, error) {
	row, err := s.dags.QueryRow(ctx,
		models.Dags.SelectAll().Where(
			models.Dags.Id.Eq(id),
			models.Dags.DeletedAt.IsNull(),
		))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row.GetPayload(), nil
}

// SaveDag upserts the dag: mirror columns (status) + the serialized payload. The
// tenant id is read from dag.Metadata on first insert.
func (s *Store) SaveDag(ctx context.Context, dag *primitive.Dag) error {
	scanner := (&models.Dag{
		Id:      &models.DagId{Value: dag.GetId()},
		Status:  dag.GetStatus(),
		Payload: dag,
	}).IntoPlain()

	now := time.Now()
	affected, err := s.dags.Scanner().Execute(ctx,
		models.Dags.Update().Set(
			models.Dags.Status.Set(scanner.Status),
			models.Dags.Payload.Set(string(scanner.Payload)),
			models.Dags.UpdatedAt.Set(now),
		).Where(models.Dags.Id.Eq(dag.GetId())))
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}

	tenantID := dag.GetMetadata()[MetadataTenantID]
	if tenantID == "" {
		return errors.New("dagstore: cannot insert dag without metadata tenant_id")
	}
	insert := (&models.Dag{
		Id:         &models.DagId{Value: dag.GetId()},
		TenantId:   &models.TenantId{Value: tenantID},
		Status:     dag.GetStatus(),
		Payload:    dag,
		Timestamps: &models.Timestamps{CreatedAt: timestamppb.New(now), UpdatedAt: timestamppb.New(now)},
	}).IntoPlain()
	_, err = s.dags.Scanner().Execute(ctx, models.Dags.Insert().From(insert.AllSetters()...))
	return err
}
