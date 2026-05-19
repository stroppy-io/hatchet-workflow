package iam

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	"github.com/stroppy-io/stroppy-cloud/internal/core/eventing"
	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	"github.com/stroppy-io/stroppy-cloud/internal/core/tracing"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/pgtx"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateUser persists a new user with a hashed password. Returns the user
// proto with id + timestamps populated.
func (s *Service) CreateUser(ctx context.Context, user *iampb.User, password string) (*iampb.User, error) {
	return tracing.WithTraceRet(s.Tracer(), ctx, "CreateUser",
		func(ctx context.Context, _ trace.Span) (*iampb.User, error) {
			return pgtx.WithSerializableRet(ctx, s.txManager,
				func(ctx context.Context) (*iampb.User, error) {
					hash, err := hashPassword(password)
					if err != nil {
						return nil, err
					}
					now := time.Now()
					user.Id = &iampb.UserId{Value: ids.New()}
					user.Timestamps = &commonpb.Timestamps{
						CreatedAt: timestamppb.New(now),
						UpdatedAt: timestamppb.New(now),
					}
					scanner := user.IntoPlain()
					scanner.PasswordHash = hash

					if _, err := s.userRepo.Execute(ctx,
						iampb.Users.Insert().From(scanner.AllSetters()...),
					); err != nil {
						if iampb.IsUserEmailUniqueIdxError(err) {
							return nil, domainerr.AlreadyExists(domainerr.ResourceInfo("user", user.GetEmail()))
						}
						if iampb.IsUserNicknameUniqueIdxError(err) {
							return nil, domainerr.AlreadyExists(domainerr.ResourceInfo("user", user.GetNickname()))
						}
						return nil, err
					}
					_ = s.events.Publish(ctx, eventing.Event{
						Topic:   eventing.TopicUserCreated,
						Payload: eventing.UserCreated{UserID: user.GetId().GetValue()},
					})
					return user, nil
				})
		})
}

// GetUserByID retrieves a user, returning domainerr.NotFound if absent.
func (s *Service) GetUserByID(ctx context.Context, id *iampb.UserId) (*iampb.User, error) {
	u, err := s.userRepo.QueryRow(ctx,
		iampb.Users.SelectAll().Where(
			iampb.Users.Id.Eq(id.GetValue()),
			iampb.Users.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("user", id.GetValue()))
		}
		return nil, err
	}
	return u, nil
}

// GetUserByEmail looks up by email.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*iampb.User, error) {
	u, err := s.userRepo.QueryRow(ctx,
		iampb.Users.SelectAll().Where(
			iampb.Users.Email.Eq(email),
			iampb.Users.DeletedAt.IsNull(),
		),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.NotFound(domainerr.ResourceInfo("user", email))
		}
		return nil, err
	}
	return u, nil
}

// ListUsers returns all non-deleted users (caller is platform admin).
func (s *Service) ListUsers(ctx context.Context) ([]*iampb.User, error) {
	return s.userRepo.Query(ctx,
		iampb.Users.SelectAll().Where(iampb.Users.DeletedAt.IsNull()),
	)
}

// DeleteUser hard-deletes a user by ID (cascades on FKs).
func (s *Service) DeleteUser(ctx context.Context, id *iampb.UserId) (*iampb.User, error) {
	u, err := s.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.userRepo.Execute(ctx,
		iampb.Users.Delete().Where(iampb.Users.Id.Eq(id.GetValue())),
	); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateUser patches mutable fields (email, nickname). Password lives behind
// UpdatePassword / ResetUserPassword; tenant_members live behind member APIs.
func (s *Service) UpdateUser(ctx context.Context, user *iampb.User) (*iampb.User, error) {
	if user.GetId() == nil || user.GetId().GetValue() == "" {
		return nil, domainerr.InvalidArgument(domainerr.FieldViolation("id", "is required"))
	}
	existing, err := s.GetUserByID(ctx, user.GetId())
	if err != nil {
		return nil, err
	}
	if user.GetEmail() != "" {
		existing.Email = user.GetEmail()
	}
	if user.GetNickname() != "" {
		existing.Nickname = user.GetNickname()
	}
	now := time.Now()
	if _, err := s.userRepo.Execute(ctx,
		iampb.Users.Update().
			Set(
				iampb.Users.Email.Set(existing.GetEmail()),
				iampb.Users.Nickname.Set(existing.GetNickname()),
				iampb.Users.UpdatedAt.Set(now),
			).
			Where(iampb.Users.Id.Eq(user.GetId().GetValue())),
	); err != nil {
		return nil, err
	}
	return s.GetUserByID(ctx, user.GetId())
}

// ResetUserPassword sets a new password hash and revokes all active refresh tokens.
func (s *Service) ResetUserPassword(ctx context.Context, id *iampb.UserId, newPassword string) (*iampb.User, error) {
	return pgtx.WithSerializableRet(ctx, s.txManager, func(ctx context.Context) (*iampb.User, error) {
		u, err := s.GetUserByID(ctx, id)
		if err != nil {
			return nil, err
		}
		hash, err := hashPassword(newPassword)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		if _, err := s.userRepo.Execute(ctx,
			iampb.Users.Update().
				Set(iampb.Users.PasswordHash.Set(hash)).
				Where(iampb.Users.Id.Eq(id.GetValue())),
		); err != nil {
			return nil, err
		}
		// Revoke all active refresh tokens — forces re-login on all devices.
		if _, err := s.refreshRepo.Execute(ctx,
			iampb.RefreshTokens.Update().
				Set(iampb.RefreshTokens.RevokedAt.Set(&now)).
				Where(
					iampb.RefreshTokens.UserId.Eq(id.GetValue()),
					iampb.RefreshTokens.RevokedAt.IsNull(),
				),
		); err != nil {
			return nil, err
		}
		return u, nil
	})
}

// PromoteToAdmin sets platform_role=PLATFORM_ROLE_ADMIN on the given user.
func (s *Service) PromoteToAdmin(ctx context.Context, id *iampb.UserId) (*iampb.User, error) {
	return pgtx.WithSerializableRet(ctx, s.txManager, func(ctx context.Context) (*iampb.User, error) {
		if _, err := s.userRepo.Execute(ctx,
			iampb.Users.Update().
				Set(iampb.Users.PlatformRole.Set(iampb.PlatformRole_PLATFORM_ROLE_ADMIN.String())).
				Where(iampb.Users.Id.Eq(id.GetValue())),
		); err != nil {
			return nil, err
		}
		return s.GetUserByID(ctx, id)
	})
}
