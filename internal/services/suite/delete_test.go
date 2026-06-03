package suite

import (
	"context"
	"testing"

	trm "github.com/avito-tech/go-transaction-manager/trm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func TestDeleteSuiteSoftDeletes(t *testing.T) {
	repo := &fakeSuiteRepo{
		suite: &models.SuiteRecord{
			Entity: &common.Entity{Id: "suite-1", TenantId: "tenant-1", IsFavorite: true, Timings: &common.Timings{}},
			Spec:   &domain.Suite{Id: "suite-1"},
		},
	}
	svc := NewSuiteService(SuiteDeps{Suites: repo, Tx: noopTrm{}})

	_, err := svc.DeleteSuite(context.Background(), &api.DeleteSuiteRequest{TenantId: "tenant-1", Id: "suite-1"})
	if err != nil {
		t.Fatalf("delete suite: %v", err)
	}

	if repo.suite.GetEntity().GetTimings().GetDeletedAt() == nil {
		t.Fatal("deleted_at was not set")
	}
	if repo.suite.GetEntity().GetIsFavorite() {
		t.Fatal("is_favorite was persisted")
	}
	if repo.updates != 1 {
		t.Fatalf("updates = %d, want 1", repo.updates)
	}
}

func TestDeleteSuiteAlreadyDeletedIsNoop(t *testing.T) {
	repo := &fakeSuiteRepo{
		suite: &models.SuiteRecord{
			Entity: &common.Entity{
				Id:       "suite-1",
				TenantId: "tenant-1",
				Timings:  &common.Timings{DeletedAt: timestamppb.Now()},
			},
		},
	}
	svc := NewSuiteService(SuiteDeps{Suites: repo, Tx: noopTrm{}})

	_, err := svc.DeleteSuite(context.Background(), &api.DeleteSuiteRequest{TenantId: "tenant-1", Id: "suite-1"})
	if err != nil {
		t.Fatalf("delete suite: %v", err)
	}
	if repo.updates != 0 {
		t.Fatalf("updates = %d, want 0", repo.updates)
	}
}

func TestStartSuiteRejectsDeletedStoredSuite(t *testing.T) {
	launcher := &fakeSuiteLauncher{}
	repo := &fakeSuiteRepo{
		suite: &models.SuiteRecord{
			Entity: &common.Entity{
				Id:       "suite-1",
				TenantId: "tenant-1",
				Timings:  &common.Timings{DeletedAt: timestamppb.Now()},
			},
			Spec: &domain.Suite{Id: "suite-1"},
		},
	}
	svc := NewSuiteService(SuiteDeps{Authn: fakeAuthn{}, Suites: repo, Launcher: launcher, Tx: noopTrm{}})

	_, err := svc.StartSuite(context.Background(), &api.StartSuiteRequest{
		TenantId: "tenant-1",
		Source:   &api.StartSuiteRequest_SuiteId{SuiteId: "suite-1"},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("status = %s, want %s; err=%v", status.Code(err), codes.FailedPrecondition, err)
	}
	if launcher.validates != 0 {
		t.Fatalf("launcher validates = %d, want 0", launcher.validates)
	}
}

type fakeAuthn struct{}

func (fakeAuthn) Caller(context.Context) (*iam.AccessClaims, error) {
	return &iam.AccessClaims{AccountId: "account-1"}, nil
}

type noopTrm struct{}

func (noopTrm) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (noopTrm) DoWithSettings(ctx context.Context, _ trm.Settings, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type fakeSuiteRepo struct {
	suite   *models.SuiteRecord
	updates int
}

func (*fakeSuiteRepo) Create(context.Context, *models.SuiteRecord) error {
	return nil
}

func (r *fakeSuiteRepo) Get(_ context.Context, tenantID, id string) (*models.SuiteRecord, error) {
	if r.suite == nil || r.suite.GetEntity().GetTenantId() != tenantID || r.suite.GetEntity().GetId() != id {
		return nil, derrors.ErrNotFound
	}
	return r.suite, nil
}

func (*fakeSuiteRepo) List(context.Context, SuiteListQuery) ([]*models.SuiteRecord, string, error) {
	return nil, "", nil
}

func (*fakeSuiteRepo) ListFacets(context.Context, SuiteFacetQuery) (*api.ListSuiteFacetsResponse, error) {
	return &api.ListSuiteFacetsResponse{}, nil
}

func (r *fakeSuiteRepo) Update(_ context.Context, suite *models.SuiteRecord) error {
	r.updates++
	r.suite = suite
	return nil
}

type fakeSuiteLauncher struct {
	validates int
}

func (l *fakeSuiteLauncher) Validate(context.Context, string, *domain.Suite) error {
	l.validates++
	return nil
}

func (*fakeSuiteLauncher) Launch(context.Context, *models.SuiteRunRecord, *domain.Suite) (func(context.Context) error, error) {
	return nil, nil
}
