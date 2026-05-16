package ops

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

// BinaryCacheService resolves artifact download requests against the binary_artifacts table.
type BinaryCacheService struct {
	repo  *repository.ProtoRepository[opspb.BinaryArtifactAlias, opspb.BinaryArtifactColumnAlias, *opspb.BinaryArtifactScanner, *opspb.BinaryArtifact]
	txMgr pgtx.TxManager
}

// NewBinaryCacheService constructs a BinaryCacheService.
func NewBinaryCacheService(executor exec.DB, txMgr pgtx.TxManager) *BinaryCacheService {
	return &BinaryCacheService{
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(opspb.BinaryArtifacts.Table, executor),
			opspb.BinaryArtifactConverter,
		),
		txMgr: txMgr,
	}
}

// Resolve looks up a BinaryArtifact by (name, version, filename).
// Returns NotFound if no matching row exists (deleted rows are excluded).
func (s *BinaryCacheService) Resolve(ctx context.Context, req *agentpb.ResolveArtifactRequest) (*agentpb.ResolveArtifactResponse, error) {
	where := opspb.BinaryArtifacts.SelectAll().Where(
		opspb.BinaryArtifacts.Name.Eq(req.GetName()),
		opspb.BinaryArtifacts.Version.Eq(req.GetVersion()),
		opspb.BinaryArtifacts.Filename.Eq(req.GetFilename()),
		opspb.BinaryArtifacts.DeletedAt.IsNull(),
	)

	artifact, err := s.repo.QueryRow(ctx, where)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(
				domainerr.ResourceInfo("binary_artifact",
					req.GetName()+"/"+req.GetVersion()+"/"+req.GetFilename()),
			)
		}
		return nil, err
	}

	return &agentpb.ResolveArtifactResponse{
		Artifact:    artifact,
		DownloadUrl: artifact.GetStorageUri(),
	}, nil
}
