package iam

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

// UpdatePassword changes the password for the supplied user. The caller
// supplies their old password; mismatch returns Unauthenticated to avoid
// account enumeration. new and confirmation must match.
func (s *Service) UpdatePassword(ctx context.Context, userID *iampb.UserId, oldPassword, newPassword, confirmation string) (*iampb.User, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "UpdatePassword",
		func(ctx context.Context, _ trace.Span) (*iampb.User, error) {
			if newPassword != confirmation {
				return nil, domainerr.InvalidArgument(domainerr.FieldViolation("new_password_confirmation", "does not match new_password"))
			}
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.User, error) {
					scanner, err := s.userRepo.Scanner().QueryRow(ctx,
						iampb.Users.SelectAll().Where(
							iampb.Users.Id.Eq(userID.GetValue()),
							iampb.Users.DeletedAt.IsNull(),
						),
					)
					if err != nil {
						if errors.Is(err, pgx.ErrNoRows) {
							return nil, domainerr.NotFound(domainerr.ResourceInfo("user", userID.GetValue()))
						}
						return nil, err
					}
					if !verifyPassword(scanner.PasswordHash, oldPassword) {
						return nil, domainerr.Unauthenticated()
					}
					hash, err := hashPassword(newPassword)
					if err != nil {
						return nil, err
					}
					now := timestamppb.Now()
					scanner.PasswordHash = hash
					scanner.UpdatedAt = now.AsTime()
					if _, err := s.userRepo.Execute(ctx,
						iampb.Users.Update().
							Set(
								scanner.GetSetter(iampb.UserColumnPasswordHash)(),
								scanner.GetSetter(iampb.UserColumnUpdatedAt)(),
							).
							Where(iampb.Users.Id.Eq(userID.GetValue())),
					); err != nil {
						return nil, err
					}
					return scanner.IntoPb(), nil
				})
		})
}
