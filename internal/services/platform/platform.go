// Package platform implements the GLOBAL (singleton) control-plane settings —
// PlatformAdminService. Root-admin only (gated by the is_admin guard at
// registration). The single server_addr handed to every agent lives here; an empty
// value means "derive a docker-host fallback" (resolved by services/deploy).
package platform

import (
	"context"
	"errors"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/jackc/pgx/v5"
	"github.com/yaroher/ratel/pkg/exec"
	"github.com/yaroher/ratel/pkg/repository"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	adminapi "github.com/stroppy-io/stroppy-cloud/internal/api/admin"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// SingletonID is the fixed primary key of the one platform_settings row.
const SingletonID = "platform"

// Service implements the platform-admin (singleton) settings.
type Service struct {
	*tracing.Entity
	repo *repository.ProtoRepository[
		models.PlatformSettingsAlias,
		models.PlatformSettingsColumnAlias,
		*models.PlatformSettingsScanner,
		*models.PlatformSettings,
	]
	txm tx.Trm
}

var _ adminapi.PlatformAdminActions = (*Service)(nil)

func New(logger *xlog.Logger, executor exec.DB, txm tx.Trm) *Service {
	return &Service{
		Entity: tracing.NewEntity(logger.AppendName("PlatformService")),
		repo: repository.NewProtoRepository(
			repository.NewScannerRepository(models.PlatformSettingss.Table, executor),
			models.PlatformSettingsConverter,
		),
		txm: txm,
	}
}

// GetPlatformSettings returns the singleton; an unset platform returns an empty
// PlatformSettings (server_addr == "") rather than NotFound.
func (s *Service) GetPlatformSettings(ctx context.Context, _ *emptypb.Empty) (*models.PlatformSettings, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "GetPlatformSettings",
		func(ctx context.Context, _ trace.Span) (*models.PlatformSettings, error) {
			row, err := s.repo.QueryRow(ctx,
				models.PlatformSettingss.SelectAll().Where(models.PlatformSettingss.Id.Eq(SingletonID)))
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return &models.PlatformSettings{Id: SingletonID}, nil
				}
				return nil, status.Errorf(codes.Internal, "get platform settings: %v", err)
			}
			return row, nil
		})
}

// SetPlatformSettings upserts the singleton server_addr (root-admin only).
func (s *Service) SetPlatformSettings(ctx context.Context, req *adminpb.SetPlatformSettingsRequest) (*models.PlatformSettings, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "SetPlatformSettings",
		func(ctx context.Context, _ trace.Span) (*models.PlatformSettings, error) {
			now := time.Now()
			scanner := (&models.PlatformSettings{Id: SingletonID, ServerAddr: req.GetServerAddr()}).IntoPlain()
			scanner.CreatedAt = now
			scanner.UpdatedAt = now
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.PlatformSettings, error) {
				updated, err := s.repo.QueryRow(ctx,
					models.PlatformSettingss.Insert().From(scanner.AllSetters()...).
						OnConflict(models.PlatformSettingsColumnId).
						DoUpdate(models.PlatformSettingsColumnServerAddr, models.PlatformSettingsColumnUpdatedAt).
						ReturningAll())
				if err != nil {
					return nil, status.Errorf(codes.Internal, "set platform settings: %v", err)
				}
				return updated, nil
			})
		})
}
