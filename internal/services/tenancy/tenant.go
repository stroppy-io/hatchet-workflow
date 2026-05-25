package tenancy

import (
	"context"
	"time"

	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
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
	"github.com/stroppy-io/stroppy-cloud/internal/services/svcutil"
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
	accounts *repository.ProtoRepository[
		models.AccountAlias,
		models.AccountColumnAlias,
		*models.AccountScanner,
		*models.Account,
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
		accounts: repository.NewProtoRepository(
			repository.NewScannerRepository(models.Accounts.Table, executor),
			models.AccountConverter,
		),
		authz: az,
		txm:   txm,
	}
}

// loadAccounts hydrates the member accounts referenced by page in a single
// WHERE id IN (...) query, keyed by account id (no N+1). Soft-deleted accounts
// are skipped (the row's Account stays nil).
func (s *TenantService) loadAccounts(ctx context.Context, page []*models.TenantMember) (map[string]*models.Account, error) {
	if len(page) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(page))
	seen := make(map[string]struct{}, len(page))
	for _, m := range page {
		id := m.GetAccountId().GetValue()
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	accounts, err := s.accounts.Query(ctx,
		models.Accounts.SelectAll().Where(
			models.Accounts.Id.In(ids...),
			models.Accounts.DeletedAt.IsNull(),
		))
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*models.Account, len(accounts))
	for _, a := range accounts {
		byID[a.GetEntity().GetId().GetValue()] = a
	}
	return byID, nil
}

// ListMyTenants returns the tenants the calling account is a member of, loaded
// in a single WHERE id IN (...) query (no N+1).
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
			if len(memberships) == 0 {
				return out, nil
			}
			ids := make([]string, 0, len(memberships))
			seen := make(map[string]struct{}, len(memberships))
			for _, m := range memberships {
				id := m.GetTenantId().GetValue()
				if _, dup := seen[id]; dup {
					continue
				}
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
			tenants, err := s.tenants.Query(ctx,
				models.Tenants.SelectAll().Where(
					models.Tenants.Id.In(ids...),
					models.Tenants.DeletedAt.IsNull(),
				))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "load tenants: %v", err)
			}
			out.Tenants = tenants
			return out, nil
		})
}

// ListTenantMembers returns the tenant's members with cursor pagination
// (newest-first by default), optionally filtered by role. Read requires VIEWER.
func (s *TenantService) ListTenantMembers(ctx context.Context, req *uipb.ListTenantMembersRequest) (*uipb.ListTenantMembersResponse, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "ListTenantMembers",
		func(ctx context.Context, _ trace.Span) (*uipb.ListTenantMembersResponse, error) {
			c, _ := caller.FromContext(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_VIEWER); err != nil {
				return nil, err
			}
			size := svcutil.PageSize(req.GetPage())
			desc := svcutil.CursorDesc(req.GetOrder())
			q := models.TenantMembers.SelectAll().Where(
				models.TenantMembers.TenantId.Eq(req.GetTenantId().GetValue()),
				models.TenantMembers.DeletedAt.IsNull(),
			)
			if req.Role != nil {
				// Role is stored as the enum String() value (tenant_plain converter).
				q = q.Where(models.TenantMembers.Role.Eq(req.GetRole().String()))
			}
			// search has no backing column on tenant_members (members are
			// account_id + role only); name/email search would require hydrating
			// the joined Account — tracked with the hydration follow-up below.
			// tags: Tags is serialized JSON of common.Tags (a TEXT column) — match
			// each requested free tag and key=value label as a substring.
			for _, tag := range req.GetTags().GetTags() {
				q = q.Where(models.TenantMembers.Tags.ILike("%" + tag + "%"))
			}
			for k, v := range req.GetTags().GetLabels() {
				q = q.Where(models.TenantMembers.Tags.ILike("%" + k + "%" + v + "%"))
			}
			if tok := req.GetPage().GetToken(); tok != "" {
				if desc {
					q = q.Where(models.TenantMembers.Id.Lt(tok))
				} else {
					q = q.Where(models.TenantMembers.Id.Gt(tok))
				}
			}
			if desc {
				q = q.OrderByDESC(models.TenantMemberColumnId)
			} else {
				q = q.OrderByASC(models.TenantMemberColumnId)
			}
			members, err := s.members.Query(ctx, q.Limit(size+1))
			if err != nil {
				return nil, status.Errorf(codes.Internal, "list tenant members: %v", err)
			}
			page, pageInfo := svcutil.Paginate(members, size, func(m *models.TenantMember) string {
				return m.GetEntity().GetId().GetValue()
			})
			// Hydrate TenantMemberRow.Account (email/nickname) for display: load
			// all referenced accounts in a single WHERE id IN (...) query (no N+1).
			accByID, err := s.loadAccounts(ctx, page)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "hydrate member accounts: %v", err)
			}
			rows := make([]*uipb.TenantMemberRow, 0, len(page))
			for _, m := range page {
				rows = append(rows, &uipb.TenantMemberRow{
					Member:  m,
					Account: accByID[m.GetAccountId().GetValue()],
				})
			}
			return &uipb.ListTenantMembersResponse{Members: rows, PageInfo: pageInfo}, nil
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

// UpdateMemberRole changes an existing membership's role (OWNER only).
func (s *TenantService) UpdateMemberRole(ctx context.Context, req *uipb.UpdateMemberRoleRequest) (*models.TenantMember, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "UpdateMemberRole",
		func(ctx context.Context, _ trace.Span) (*models.TenantMember, error) {
			c, _ := caller.FromContext(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			now := time.Now()
			updated, err := s.members.QueryRow(ctx,
				models.TenantMembers.Update().Set(
					// Role is stored as the enum String() value (tenant_plain converter).
					models.TenantMembers.Role.Set(req.GetRole().String()),
					models.TenantMembers.UpdatedAt.Set(now),
				).Where(
					models.TenantMembers.TenantId.Eq(req.GetTenantId().GetValue()),
					models.TenantMembers.AccountId.Eq(req.GetAccountId().GetValue()),
					models.TenantMembers.DeletedAt.IsNull(),
				).ReturningAll(),
			)
			if err != nil {
				return nil, svcutil.NotFound(err, "member")
			}
			return updated, nil
		})
}

// LookupAccountByEmail resolves an account by EXACT email so an OWNER can add a
// member without the ULID (OWNER only; exact match or NotFound — no enumeration).
func (s *TenantService) LookupAccountByEmail(ctx context.Context, req *uipb.LookupAccountByEmailRequest) (*models.Account, error) {
	return tracing.WithTraceRetErr(s.Tracer(), ctx, "LookupAccountByEmail",
		func(ctx context.Context, _ trace.Span) (*models.Account, error) {
			c, _ := caller.FromContext(ctx)
			if err := s.authz.Require(ctx, c, req.GetTenantId(), models.TenantMember_ROLE_OWNER); err != nil {
				return nil, err
			}
			account, err := s.accounts.QueryRow(ctx, models.Accounts.SelectAll().Where(
				models.Accounts.Email.Eq(req.GetEmail()),
				models.Accounts.DeletedAt.IsNull(),
			))
			if err != nil {
				return nil, svcutil.NotFound(err, "account")
			}
			return account, nil
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
