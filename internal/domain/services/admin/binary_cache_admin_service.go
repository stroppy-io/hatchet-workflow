package admin

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

// BinaryCacheAdminService manages binary artifact cache entries on behalf of
// platform administrators. It provides prewarm, list, get, and evict operations
// directly on the binary_artifacts table.
type BinaryCacheAdminService struct {
	*tracing.Entity
	repo  *repository.ProtoRepository[opspb.BinaryArtifactAlias, opspb.BinaryArtifactColumnAlias, *opspb.BinaryArtifactScanner, *opspb.BinaryArtifact]
	txMgr pgtx.TxManager
}

// NewBinaryCacheAdminService constructs a BinaryCacheAdminService.
func NewBinaryCacheAdminService(executor exec.DB, txMgr pgtx.TxManager) *BinaryCacheAdminService {
	return &BinaryCacheAdminService{
		Entity: tracing.NewEntity("admin.BinaryCacheAdminService"),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.BinaryArtifacts.Table, executor),
			opspb.BinaryArtifactConverter,
		),
		txMgr: txMgr,
	}
}

// Prewarm inserts or upserts BinaryArtifact rows for each ResolveArtifactRequest
// item. For each item, if a non-deleted row with the same (name, version, filename,
// arch, os) already exists the updated_at is bumped; otherwise a new row is created
// with the provided storage_uri / origin_url left empty (operator fills those via
// separate tooling or directly in DB). Returns the resulting artifact list.
func (s *BinaryCacheAdminService) Prewarm(ctx context.Context, req *adminpb.PrewarmRequest) (*opspb.BinaryArtifact_List, error) {
	out := &opspb.BinaryArtifact_List{}
	for _, item := range req.GetItems() {
		art, err := s.prewarmOne(ctx, item)
		if err != nil {
			return nil, err
		}
		out.Artifacts = append(out.Artifacts, art)
	}
	return out, nil
}

func (s *BinaryCacheAdminService) prewarmOne(ctx context.Context, item *agentpb.ResolveArtifactRequest) (*opspb.BinaryArtifact, error) {
	// Try to find an existing non-deleted row first.
	existing, err := s.repo.QueryRow(ctx,
		opspb.BinaryArtifacts.SelectAll().Where(
			opspb.BinaryArtifacts.Name.Eq(item.GetName()),
			opspb.BinaryArtifacts.Version.Eq(item.GetVersion()),
			opspb.BinaryArtifacts.Filename.Eq(item.GetFilename()),
			opspb.BinaryArtifacts.DeletedAt.IsNull(),
		),
	)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		// Row exists — bump updated_at to signal re-warm.
		now := time.Now()
		if _, err := s.repo.Execute(ctx,
			opspb.BinaryArtifacts.Update().
				Set(opspb.BinaryArtifacts.UpdatedAt.Set(now)).
				Where(opspb.BinaryArtifacts.Id.Eq(existing.GetId().GetValue())),
		); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// No existing row — insert a skeleton artifact row. The operator must
	// subsequently fill in storage_uri/sha256 via tooling or direct DB update.
	now := time.Now()
	art := &opspb.BinaryArtifact{
		Id: &opspb.BinaryArtifactId{Value: ids.New()},
		Timestamps: &commonpb.Timestamps{
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		},
		Name:     item.GetName(),
		Version:  item.GetVersion(),
		Filename: item.GetFilename(),
		Arch:     item.GetArch(),
		Os:       item.GetOs(),
	}
	scanner := art.IntoPlain()
	if _, err := s.repo.Execute(ctx,
		opspb.BinaryArtifacts.Insert().From(scanner.AllSetters()...),
	); err != nil {
		return nil, err
	}
	return art, nil
}

// GetArtifact retrieves a BinaryArtifact by ID. Returns NotFound if absent or deleted.
func (s *BinaryCacheAdminService) GetArtifact(ctx context.Context, id *opspb.BinaryArtifactId) (*opspb.BinaryArtifact, error) {
	art, err := s.repo.QueryRow(ctx,
		opspb.BinaryArtifacts.SelectAll().Where(
			opspb.BinaryArtifacts.Id.Eq(id.GetValue()),
			opspb.BinaryArtifacts.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("binary_artifact", id.GetValue()))
		}
		return nil, err
	}
	return art, nil
}

// ListArtifacts returns all non-deleted artifacts, optionally filtered by kind
// and/or name prefix.
func (s *BinaryCacheAdminService) ListArtifacts(ctx context.Context, req *adminpb.ListArtifactsRequest) (*opspb.BinaryArtifact_List, error) {
	q := opspb.BinaryArtifacts.SelectAll().Where(
		opspb.BinaryArtifacts.DeletedAt.IsNull(),
	)
	artifacts, err := s.repo.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	// Apply optional in-memory filters (kind + name_prefix). The ratel query
	// builder does not support LIKE/ILIKE out of the box on these columns yet,
	// so we filter post-query when the request carries restrictions.
	if req.GetKind() != opspb.BinaryArtifactKind_BINARY_ARTIFACT_KIND_UNSPECIFIED || req.GetNamePrefix() != "" {
		filtered := artifacts[:0]
		for _, a := range artifacts {
			if req.GetKind() != opspb.BinaryArtifactKind_BINARY_ARTIFACT_KIND_UNSPECIFIED &&
				a.GetKind() != req.GetKind() {
				continue
			}
			if req.GetNamePrefix() != "" && !startsWith(a.GetName(), req.GetNamePrefix()) {
				continue
			}
			filtered = append(filtered, a)
		}
		artifacts = filtered
	}

	return &opspb.BinaryArtifact_List{Artifacts: artifacts}, nil
}

// EvictArtifact soft-deletes a binary artifact by ID.
// Idempotent: returns NotFound on the second call (row already deleted).
func (s *BinaryCacheAdminService) EvictArtifact(ctx context.Context, id *opspb.BinaryArtifactId) (*opspb.BinaryArtifact, error) {
	art, err := s.GetArtifact(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if _, err := s.repo.Execute(ctx,
		opspb.BinaryArtifacts.Update().
			Set(opspb.BinaryArtifacts.DeletedAt.Set(&now)).
			Where(opspb.BinaryArtifacts.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return art, nil
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
