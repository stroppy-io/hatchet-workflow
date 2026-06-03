package preset

import (
	"context"
	"testing"

	trm "github.com/avito-tech/go-transaction-manager/trm"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDeleteDatabasePresetSoftDeletes(t *testing.T) {
	repo := &fakeDatabasePresetRepo{
		preset: &models.DatabasePresetRecord{
			Entity:   &common.Entity{Id: "preset-1", TenantId: "tenant-1", IsFavorite: true, Timings: &common.Timings{}},
			IsSystem: false,
		},
	}
	svc := NewDatabasePresetService(Deps{Authn: fakeAuthn{}, Databases: repo, Tx: noopTrm{}})

	_, err := svc.DeleteDatabasePreset(context.Background(), &api.DeleteDatabasePresetRequest{
		TenantId: "tenant-1",
		Id:       "preset-1",
	})
	if err != nil {
		t.Fatalf("delete database preset: %v", err)
	}

	if repo.preset.GetEntity().GetTimings().GetDeletedAt() == nil {
		t.Fatal("deleted_at was not set")
	}
	if repo.preset.GetEntity().GetIsFavorite() {
		t.Fatal("is_favorite was persisted")
	}
	if repo.updates != 1 {
		t.Fatalf("updates = %d, want 1", repo.updates)
	}
}

func TestDeleteDatabasePresetAlreadyDeletedIsNoop(t *testing.T) {
	deletedAt := timestamppb.Now()
	repo := &fakeDatabasePresetRepo{
		preset: &models.DatabasePresetRecord{
			Entity: &common.Entity{
				Id:       "preset-1",
				TenantId: "tenant-1",
				Timings:  &common.Timings{DeletedAt: deletedAt},
			},
		},
	}
	svc := NewDatabasePresetService(Deps{Authn: fakeAuthn{}, Databases: repo, Tx: noopTrm{}})

	_, err := svc.DeleteDatabasePreset(context.Background(), &api.DeleteDatabasePresetRequest{
		TenantId: "tenant-1",
		Id:       "preset-1",
	})
	if err != nil {
		t.Fatalf("delete database preset: %v", err)
	}
	if repo.updates != 0 {
		t.Fatalf("updates = %d, want 0", repo.updates)
	}
}

func TestPreserveEntityKeepsDeletedAt(t *testing.T) {
	deletedAt := timestamppb.Now()
	out := preserveEntity(
		&common.Entity{
			Id:       "preset-1",
			TenantId: "tenant-1",
			AuthorId: "author-1",
			Timings:  &common.Timings{CreatedAt: timestamppb.Now(), DeletedAt: deletedAt},
		},
		&common.Entity{Name: "new name"},
		Deps{},
	)

	if out.GetTimings().GetDeletedAt() != deletedAt {
		t.Fatal("deleted_at was not preserved")
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

type fakeDatabasePresetRepo struct {
	preset  *models.DatabasePresetRecord
	updates int
}

func (*fakeDatabasePresetRepo) Create(context.Context, *models.DatabasePresetRecord) error {
	return nil
}

func (r *fakeDatabasePresetRepo) Get(_ context.Context, tenantID, id, _ string) (*models.DatabasePresetRecord, error) {
	if r.preset == nil || r.preset.GetEntity().GetTenantId() != tenantID || r.preset.GetEntity().GetId() != id {
		return nil, derrors.ErrNotFound
	}
	return r.preset, nil
}

func (*fakeDatabasePresetRepo) List(context.Context, *api.ListDatabasePresetsRequest, string) ([]*models.DatabasePresetRecord, string, error) {
	return nil, "", nil
}

func (r *fakeDatabasePresetRepo) Update(_ context.Context, preset *models.DatabasePresetRecord) error {
	r.updates++
	r.preset = preset
	return nil
}
