package postgres

import (
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFilterPackageRecordsHonorsEntityFacetAndPage(t *testing.T) {
	query := packages.PackageQuery{
		TenantID: "tenant-1",
		Filter:   &common.EntityFilter{Search: "postgres"},
		Formats:  []models.PackageRecord_Format{models.PackageRecord_FORMAT_DEB},
		DbKinds:  []domain.Database_Kind{domain.Database_KIND_POSTGRES},
		Sort:     &common.EntitySort{Field: common.EntitySortField_ENTITY_SORT_FIELD_NAME},
		PageSize: 1,
	}
	records := []*models.PackageRecord{
		pkg("z", "Z postgres", models.PackageRecord_FORMAT_DEB, domain.Database_KIND_POSTGRES),
		pkg("a", "A postgres", models.PackageRecord_FORMAT_DEB, domain.Database_KIND_POSTGRES),
		pkg("mysql", "mysql", models.PackageRecord_FORMAT_DEB, domain.Database_KIND_MYSQL),
		pkg("binary", "postgres binary", models.PackageRecord_FORMAT_BINARY, domain.Database_KIND_POSTGRES),
	}

	page, next := filterPackageRecords(records, query)
	if len(page) != 1 || page[0].GetEntity().GetId() != "a" {
		t.Fatalf("first page = %v, want a", ids(page))
	}
	if next == "" {
		t.Fatal("next page token is empty")
	}
	query.PageToken = next
	page, next = filterPackageRecords(records, query)
	if len(page) != 1 || page[0].GetEntity().GetId() != "z" {
		t.Fatalf("second page = %v, want z", ids(page))
	}
	if next != "" {
		t.Fatalf("unexpected next token %q", next)
	}
}

func pkg(id, name string, format models.PackageRecord_Format, kind domain.Database_Kind) *models.PackageRecord {
	return &models.PackageRecord{
		Entity:       entity(id, name),
		Format:       format,
		TargetDbKind: kind,
	}
}

func entity(id, name string) *common.Entity {
	now := timestamppb.New(time.Unix(100, 0))
	return &common.Entity{
		Id:       id,
		TenantId: "tenant-1",
		Name:     name,
		Timings:  &common.Timings{CreatedAt: now, UpdatedAt: now},
	}
}

func ids[T entityRecord](records []T) []string {
	out := make([]string, 0, len(records))
	for _, rec := range records {
		out = append(out, rec.GetEntity().GetId())
	}
	return out
}
