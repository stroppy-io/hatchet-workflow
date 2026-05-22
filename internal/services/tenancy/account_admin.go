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
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
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

// ListAccounts returns the platform's accounts (cross-tenant; gated by the
// is_admin interceptor) with cursor pagination (newest-first by default),
// optionally filtered by the platform-admin flag.
func (s *AccountAdminService) ListAccounts(ctx context.Context, req *adminpb.ListAccountsRequest) (*adminpb.ListAccountsResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListAccounts",
		func(ctx context.Context, _ trace.Span) (*adminpb.ListAccountsResponse, error) {
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.Accounts.SelectAll().Where(
				models.Accounts.DeletedAt.IsNull(),
			)
			if req.IsAdmin != nil {
				q = q.Where(models.Accounts.IsAdmin.Eq(req.GetIsAdmin()))
			}
			// search: free text over email / nickname.
			if req.GetSearch() != "" {
				pat := "%" + req.GetSearch() + "%"
				q = q.Where(models.Accounts.Or(
					models.Accounts.Email.ILike(pat),
					models.Accounts.Nickname.ILike(pat),
				))
			}
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.Accounts.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.Accounts.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.Accounts.Id.Lt(tok))
				} else {
					q = q.Where(models.Accounts.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.AccountColumnId)
			} else {
				q = q.OrderByASC(models.AccountColumnId)
			}
			rows, err := s.accounts.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list accounts: %v", err)
			}
			items, pageInfo := svcutil.Paginate(rows, size, func(a *models.Account) string {
				return a.GetEntity().GetId().GetValue()
			})
			return &adminpb.ListAccountsResponse{Accounts: items, PageInfo: pageInfo}, nil
		})
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

// accountUpdatableColumns maps proto field paths (UpdateAccountRequest.account)
// to the mutable account columns the mask may select. id/created_at and the
// write-only password_hash are never writable here (password has its own RPC).
var accountUpdatableColumns = map[string]models.AccountColumnAlias{
	"email":    models.AccountColumnEmail,
	"nickname": models.AccountColumnNickname,
	"is_admin": models.AccountColumnIsAdmin,
	"isAdmin":  models.AccountColumnIsAdmin,
	"tags":     models.AccountColumnTags,
}

// UpdateAccount updates the mutable account fields. When req.update_mask names
// paths, only those columns are written; an empty/nil mask is full-replace of
// the mutable fields (back-compat). updated_at is always bumped.
func (s *AccountAdminService) UpdateAccount(ctx context.Context, req *adminpb.UpdateAccountRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateAccount",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			acc := req.GetAccount()
			id := acc.GetEntity().GetId().GetValue()
			if id == "" {
				return nil, status.Error(codes.InvalidArgument, "account id required")
			}
			scanner := acc.IntoPlain()
			setters := maskedSetters(
				scanner.GetSetter,
				req.GetUpdateMask().GetPaths(),
				accountUpdatableColumns,
				// nil/empty mask = original full-replace set (back-compat):
				// email/nickname/is_admin. tags is mask-only.
				[]models.AccountColumnAlias{
					models.AccountColumnEmail,
					models.AccountColumnNickname,
					models.AccountColumnIsAdmin,
				},
			)
			setters = append(setters, models.Accounts.UpdatedAt.Set(time.Now()))
			if _, err := s.accounts.Execute(ctx,
				models.Accounts.Update().Set(setters...).
					Where(models.Accounts.Id.Eq(id)),
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

// UpdatePassword resets the target account's password (platform-admin). The
// target is req.account_id; the RPC is gated by the is_admin interceptor.
func (s *AccountAdminService) UpdatePassword(ctx context.Context, req *adminpb.UpdatePasswordRequest) (*emptypb.Empty, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdatePassword",
		func(ctx context.Context, _ trace.Span) (*emptypb.Empty, error) {
			targetID := req.GetAccountId().GetValue()
			if targetID == "" {
				return nil, status.Error(codes.InvalidArgument, "account_id required")
			}
			hash, err := domainauth.HashPassword(req.GetNewPassword())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "hash password: %v", err)
			}
			if _, err := s.accounts.Scanner().Execute(ctx,
				models.Accounts.Update().Set(
					models.Accounts.PasswordHash.Set(hash),
					models.Accounts.UpdatedAt.Set(time.Now()),
				).Where(models.Accounts.Id.Eq(targetID)),
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
