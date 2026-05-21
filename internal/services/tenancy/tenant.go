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

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	uiapi "github.com/stroppy-io/stroppy-cloud/internal/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/ids"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/authz"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/tracing"
)

// TenantService implements the tenant-scoped membership surface (ui). Member
// management requires OWNER (enforced via authz); ListMyTenants is open to any
// authenticated account (its own memberships).
type TenantService struct {
	*tracing.Entity
	tenants *repository.ProtoRepository[
		models.TenantAlias,
		models.TenantColumnAlias,
		*models.TenantScanner,
		*models.Tenant,
	]
	members *repository.ProtoRepository[
		models.TenantMemberAlias,
		models.TenantMemberColumnAlias,
		*models.TenantMemberScanner,
		*models.TenantMember,
	]
	authz *authz.Authz
	txm   tx.Trm
}

var _ uiapi.TenantActions = (*TenantService)(nil)

// NewTenantService builds the service. authz enforces per-tenant OWNER.
func NewTenantService(logger *xlog.Logger, executor exec.DB, txm tx.Trm, az *authz.Authz) *TenantService {
	return &TenantService{
		Entity: tracing.NewEntity(logger.AppendName("TenantService")),
		tenants: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Tenants.Table, executor),
			models.TenantConverter,
		),
		members: repository.NewProtoRepository(
			repository.NewScannerRepository(models.TenantMembers.Table, executor),
			models.TenantMemberConverter,
		),
		authz: az,
		txm:   txm,
	}
}

// ListMyTenants returns the tenants the calling account is a member of.
//
// TODO(tenancy): loads tenants one-by-one (N+1) — no IN-clause helper wired yet.
// Batch with a single WHERE id IN (...) once available. Reported.
func (s *TenantService) ListMyTenants(ctx context.Context, _ *emptypb.Empty) (*models.Tenant_List, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListMyTenants",
		func(ctx context.Context, _ trace.Span) (*models.Tenant_List, error) {
			c, ok := caller.FromContext(ctx)
			if !ok || c.AccountID == nil {
				return nil, status.Error(codes.Unauthenticated, "no account principal")
			}
			memberships, err := s.members.Query(ctx,
				models.TenantMembers.SelectAll().Where(
					models.TenantMembers.AccountId.Eq(c.AccountID.GetValue()),
					models.TenantMembers.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list memberships: %v", err)
			}
			out := &models.Tenant_List{Tenants: make([]*models.Tenant, 0, len(memberships))}
			for _, m := range memberships {
				tenant, err := s.tenants.QueryRow(ctx,
					models.Tenants.SelectAll().Where(
						models.Tenants.Id.Eq(m.GetTenantId().GetValue()),
						models.Tenants.DeletedAt.IsNull(),
					))
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						continue
					}
					return nil, status.Errorf(codes.Internal, "load tenant: %v", err)
				}
				out.Tenants = append(out.Tenants, tenant)
			}
			return out, nil
		})
}

// AddMemberToTenant grants an account a role in the tenant (OWNER only).
func (s *TenantService) AddMemberToTenant(ctx context.Context, req *uipb.AddMemberRequest) (*models.TenantMember, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "AddMemberToTenant",
		func(ctx context.Context, _ trace.Span) (*models.TenantMember, error) {
			c, _ := caller.FromContext(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			member := &models.TenantMember{
				Entity:    ids.NewEntity(),
				TenantId:  req.GetTenantId(),
				AccountId: req.GetAccountId(),
				Role:      req.GetRole(),
			}
			return tx.DoReadCommittedRet(ctx, s.txm, func(ctx context.Context) (*models.TenantMember, error) {
				if _, err := s.members.Execute(ctx,
					models.TenantMembers.Insert().From(member.IntoPlain().AllSetters()...)); err != nil {
					return nil, status.Errorf(codes.Internal, "add member: %v", err)
				}
				return member, nil
			})
		})
}

// RemoveMemberFromTenant soft-deletes a membership (OWNER only) and returns it.
func (s *TenantService) RemoveMemberFromTenant(ctx context.Context, req *uipb.RemoveMemberRequest) (*models.TenantMember, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "RemoveMemberFromTenant",
		func(ctx context.Context, _ trace.Span) (*models.TenantMember, error) {
			c, _ := caller.FromContext(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			now := time.Now()
			removed, err := s.members.QueryRow(ctx,
				models.TenantMembers.Update().Set(
					models.TenantMembers.DeletedAt.Set(&now),
					models.TenantMembers.UpdatedAt.Set(now),
				).Where(
					models.TenantMembers.TenantId.Eq(req.GetTenantId().GetValue()),
					models.TenantMembers.AccountId.Eq(req.GetAccountId().GetValue()),
					models.TenantMembers.DeletedAt.IsNull(),
				).ReturningAll(),
			)
			if err != nil {
				return nil, notFound(err, "remove member")
			}
			return removed, nil
		})
}
