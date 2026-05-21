// Package tenancy implements the platform-admin account/tenant services and the
// tenant-scoped membership service (area A). See features/tenancy/*.feature.
//
// Platform-admin RPCs (AccountAdminService / TenantAdminService) are gated by the
// is_admin guard interceptor at registration; per-tenant membership (TenantService)
// is enforced here via services/authz.
package tenancy

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
	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// AccountAdminService implements platform-level account management.
type AccountAdminService struct {
	*tracing.Entity
	accounts *repository.ProtoRepository[
		models.AccountAlias,
		models.AccountColumnAlias,
		*models.AccountScanner,
		*models.Account,
	]
	txm tx.Trm
}

var _ adminapi.AccountAdminActions = (*AccountAdminService)(nil)

// NewAccountAdminService builds the service over the given DB executor + tx manager.
func NewAccountAdminService(logger *xlog.Logger, executor exec.DB, txm tx.Trm) *AccountAdminService {
	return &AccountAdminService{
		Entity: tracing.NewEntity(logger.AppendName("AccountAdminService")),
		accounts: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Accounts.Table, executor),
			models.AccountConverter,
		),
		txm: txm,
	}
}

// CreateAccount mints a new account with a server-assigned id and a bcrypt-hashed
// password (stored in the write-only password_hash virtual column).
func (s *AccountAdminService) CreateAccount(ctx context.Context, req *adminpb.CreateAccountRequest) (*models.Account, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "CreateAccount",
		func(ctx context.Context, _ trace.Span) (*models.Account, error) {
			acc := req.GetAccount()
			acc.Entity = ids.NewEntity()

			hash, err := domainauth.HashPassword(req.GetPassword())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "hash password: %v", err)
			}
			scanner := acc.IntoPlain()
			scanner.PasswordHash = hash

			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.Account, error) {
				if _, err := s.accounts.Scanner().Execute(ctx,
					models.Accounts.Insert().From(scanner.AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "insert account: %v", err)
				}
				return acc, nil
			})
		})
}

// UpdateAccount updates the mutable account fields.
//
// TODO(tenancy): update_mask is NOT yet honored — all mutable fields
// (email/nickname/is_admin) are written. Wire mask->column selection. Reported.
func (s *AccountAdminService) UpdateAccount(ctx context.Context, req *adminpb.UpdateAccountRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateAccount",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			acc := req.GetAccount()
			id := acc.GetEntity().GetId().GetValue()
			if id == "" {
				return nil, status.Error(codes.InvalidArgument, "account id required")
			}
			if _, err := s.accounts.Execute(ctx,
				models.Accounts.Update().Set(
					models.Accounts.Email.Set(acc.GetEmail()),
					models.Accounts.Nickname.Set(acc.GetNickname()),
					models.Accounts.IsAdmin.Set(acc.GetIsAdmin()),
					models.Accounts.UpdatedAt.Set(time.Now()),
				).Where(models.Accounts.Id.Eq(id)),
			); err != nil {
				return nil, status.Errorf(codes.Internal, "update account: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}

// DeleteAccount soft-deletes the account.
func (s *AccountAdminService) DeleteAccount(ctx context.Context, id *models.AccountId) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "DeleteAccount",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			now := time.Now()
			if _, err := s.accounts.Execute(ctx,
				models.Accounts.Update().Set(
					models.Accounts.DeletedAt.Set(&now),
					models.Accounts.UpdatedAt.Set(now),
				).Where(models.Accounts.Id.Eq(id.GetValue())),
			); err != nil {
				return nil, status.Errorf(codes.Internal, "delete account: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}

// UpdatePassword sets a new password.
//
// TODO(tenancy): UpdatePasswordRequest carries NO target account_id (proto field
// starts at 2), so this updates the CALLER's own password. An admin "reset
// someone's password" flow needs a target account_id added to the proto. Reported.
func (s *AccountAdminService) UpdatePassword(ctx context.Context, req *adminpb.UpdatePasswordRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdatePassword",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			c, ok := caller.FromContext(ctx)
			if !ok || c.AccountID == nil {
				return nil, status.Error(codes.Unauthenticated, "no account principal")
			}
			hash, err := domainauth.HashPassword(req.GetNewPassword())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "hash password: %v", err)
			}
			if _, err := s.accounts.Scanner().Execute(ctx,
				models.Accounts.Update().Set(
					models.Accounts.PasswordHash.Set(hash),
					models.Accounts.UpdatedAt.Set(time.Now()),
				).Where(models.Accounts.Id.Eq(c.AccountID.GetValue())),
			); err != nil {
				return nil, status.Errorf(codes.Internal, "update password: %v", err)
			}
			return &emptypb.Empty{}, nil
		})
}

// notFound maps pgx.ErrNoRows to codes.NotFound.
func notFound(err error, msg string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, msg)
	}
	return status.Errorf(codes.Internal, "%s: %v", msg, err)
}
