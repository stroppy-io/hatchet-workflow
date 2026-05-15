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
