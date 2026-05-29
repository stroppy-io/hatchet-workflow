package suite

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/utils"
)

/*
	===== Suite definitions (CRUD + clone + schedule) =====
*/

// CreateSuite mints a new definition owned by the caller. The server assigns
// entity.id / tenant_id / author / timings (any client-supplied values are
// overwritten) and mirrors the spec's schedule into the denormalized summary so
// the suites table can filter/sort on it.
func (s *SuiteService) CreateSuite(ctx context.Context, req *api.CreateSuiteRequest) (*api.CreateSuiteResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	rec := req.GetSuite()
	if rec == nil || rec.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "suite spec is required")
	}
	id := uuid.NewString()
	rec.Entity = &common.Entity{
		Id:          id,
		TenantId:    req.GetTenantId(),
		Name:        rec.GetEntity().GetName(),
		Description: rec.GetEntity().GetDescription(),
		AuthorId:    c.GetAccountId(),
		Timings: &common.Timings{
			CreatedAt: s.now(),
			UpdatedAt: s.now(),
		},
	}
	rec.GetSpec().Id = id
	rec.Summary = scheduleSummary(rec.GetSpec(), nil)

	if err := s.d.Suites.Create(ctx, rec); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.CreateSuiteResponse{Suite: rec}, nil
}

// GetSuite reads one definition scoped to the tenant. NotFound covers both an
// absent row and a row owned by another tenant (the repo never returns a
// cross-tenant row).
func (s *SuiteService) GetSuite(ctx context.Context, req *api.GetSuiteRequest) (*api.GetSuiteResponse, error) {
	rec, err := s.d.Suites.Get(ctx, req.GetTenantId(), req.GetId())
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.GetSuiteResponse{Suite: rec}, nil
}

// ListSuites returns the tenant's definitions, applying the entity filter,
// provider/schedule facets, sort and page. The per-caller is_favorite flag /
// favorites_only join is scoped to the caller's account.
func (s *SuiteService) ListSuites(ctx context.Context, req *api.ListSuitesRequest) (*api.ListSuitesResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	q := SuiteListQuery{
		TenantID:  req.GetTenantId(),
		Filter:    req.GetFilter(),
		Providers: req.GetProviders(),
		Sort:      req.GetSort(),
		PageSize:  req.GetPage().GetSize(),
		PageToken: req.GetPage().GetToken(),
		CallerID:  c.GetAccountId(),
	}
	if req.ScheduleEnabled != nil {
		v := req.GetScheduleEnabled()
		q.ScheduleEnabled = &v
	}
	suites, next, err := s.d.Suites.List(ctx, q)
	if err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.ListSuitesResponse{Suites: suites, NextPageToken: next}, nil
}

// UpdateSuite wholesale-replaces a definition (idempotent: a repeated identical
// set converges). entity.id selects the row; the server preserves immutable
// fields (id, tenant_id, author, created_at) from the stored row and refreshes
// updated_at, so a client cannot move a suite to another tenant or reassign its
// author. The schedule summary is recomputed from the new spec while preserving
// the last-run facets.
func (s *SuiteService) UpdateSuite(ctx context.Context, req *api.UpdateSuiteRequest) (*api.UpdateSuiteResponse, error) {
	rec := req.GetSuite()
	if rec == nil || rec.GetSpec() == nil || rec.GetEntity().GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "suite with entity.id and spec is required")
	}
	updated, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRecord, error) {
		existing, err := s.d.Suites.Get(ctx, req.GetTenantId(), rec.GetEntity().GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		// Preserve immutable identity/ownership/creation facts from the stored row.
		rec.Entity.Id = existing.GetEntity().GetId()
		rec.Entity.TenantId = existing.GetEntity().GetTenantId()
		rec.Entity.AuthorId = existing.GetEntity().GetAuthorId()
		rec.Entity.Timings = &common.Timings{
			CreatedAt: existing.GetEntity().GetTimings().GetCreatedAt(),
			UpdatedAt: s.now(),
		}
		rec.GetSpec().Id = existing.GetEntity().GetId()
		rec.Summary = scheduleSummary(rec.GetSpec(), existing.GetSummary())
		if err := s.d.Suites.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.UpdateSuiteResponse{Suite: updated}, nil
}

// DeleteSuite removes a definition (idempotent: deleting an absent suite is a
// no-op). The repo's tenant scoping means a wrong-tenant id behaves exactly like
// an absent one.
func (s *SuiteService) DeleteSuite(ctx context.Context, req *api.DeleteSuiteRequest) (*api.DeleteSuiteResponse, error) {
	if err := derrors.IgnoreNotFound(s.d.Suites.Delete(ctx, req.GetTenantId(), req.GetId())); err != nil {
		return nil, utils.MapErr(err)
	}
	return &api.DeleteSuiteResponse{}, nil
}

// CloneSuite copies a definition into a new editable one owned by the caller:
// fresh id, caller as author, fresh timings, run history reset. The source must
// belong to the tenant. The optional name overrides the copy's name.
func (s *SuiteService) CloneSuite(ctx context.Context, req *api.CloneSuiteRequest) (*api.CloneSuiteResponse, error) {
	c, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	clone, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRecord, error) {
		src, err := s.d.Suites.Get(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		dst := proto.Clone(src).(*models.SuiteRecord)
		id := uuid.NewString()
		name := src.GetEntity().GetName()
		if req.GetName() != "" {
			name = req.GetName()
		}
		dst.Entity = &common.Entity{
			Id:          id,
			TenantId:    req.GetTenantId(),
			Name:        name,
			Description: src.GetEntity().GetDescription(),
			AuthorId:    c.GetAccountId(),
			Timings: &common.Timings{
				CreatedAt: s.now(),
				UpdatedAt: s.now(),
			},
		}
		if dst.Spec == nil {
			dst.Spec = &domain.Suite{}
		}
		dst.GetSpec().Id = id
		// A clone starts with no run history; keep only the schedule mirror.
		dst.Summary = scheduleSummary(dst.GetSpec(), nil)
		if err := s.d.Suites.Create(ctx, dst); err != nil {
			return nil, utils.MapErr(err)
		}
		return dst, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.CloneSuiteResponse{Suite: clone}, nil
}

// SetSuiteSchedule sets/replaces a suite's cron schedule + enabled flag without
// re-sending the whole definition (idempotent: setting the same schedule
// converges). It mutates only spec.schedule and the denormalized summary.
func (s *SuiteService) SetSuiteSchedule(ctx context.Context, req *api.SetSuiteScheduleRequest) (*api.SetSuiteScheduleResponse, error) {
	sched := req.GetSchedule()
	if sched == nil {
		return nil, status.Error(codes.InvalidArgument, "schedule is required")
	}
	updated, err := doTxRet(ctx, s, func(ctx context.Context) (*models.SuiteRecord, error) {
		rec, err := s.d.Suites.Get(ctx, req.GetTenantId(), req.GetId())
		if err != nil {
			return nil, utils.MapErr(err)
		}
		if rec.Spec == nil {
			rec.Spec = &domain.Suite{Id: rec.GetEntity().GetId()}
		}
		rec.GetSpec().Schedule = sched
		rec.Summary = scheduleSummary(rec.GetSpec(), rec.GetSummary())
		if rec.Entity != nil && rec.Entity.Timings != nil {
			rec.Entity.Timings.UpdatedAt = s.now()
		}
		if err := s.d.Suites.Update(ctx, rec); err != nil {
			return nil, utils.MapErr(err)
		}
		return rec, nil
	})
	if err != nil {
		return nil, err
	}
	return &api.SetSuiteScheduleResponse{Suite: updated}, nil
}

// scheduleSummary recomputes the denormalized schedule facets of a SuiteRecord
// from its spec, preserving the run-history facets (run count + last-run info)
// from prev when present. A nil schedule (or one without a cron) yields a paused
// summary.
func scheduleSummary(spec *domain.Suite, prev *models.SuiteRecord_Summary) *models.SuiteRecord_Summary {
	out := &models.SuiteRecord_Summary{}
	if prev != nil {
		out.RunCount = prev.GetRunCount()
		out.LastRunAt = prev.GetLastRunAt()
		out.LastRunStatus = prev.GetLastRunStatus()
		out.NextRunAt = prev.GetNextRunAt()
	}
	if sched := spec.GetSchedule(); sched != nil {
		out.ScheduleEnabled = sched.GetEnabled()
		out.Cron = sched.GetCron()
		if !sched.GetEnabled() {
			out.NextRunAt = nil
		}
	} else {
		out.ScheduleEnabled = false
		out.Cron = ""
		out.NextRunAt = nil
	}
	return out
}
