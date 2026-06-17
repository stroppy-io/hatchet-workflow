package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/packages"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestFilterSuiteRunRecordsHonorsFacetsSortAndPage(t *testing.T) {
	minProgress := uint32(80)
	req := &api.ListSuiteRunsRequest{
		TenantId:    "tenant-1",
		Statuses:    []common.Status{common.Status_STATUS_COMPLETED},
		Providers:   []deployment.Provider{deployment.Provider_PROVIDER_DOCKER},
		ProgressMin: &minProgress,
		Sort: &api.ListSuiteRunsRequest_Sort{
			By:   &api.ListSuiteRunsRequest_Sort_Kind_{Kind: api.ListSuiteRunsRequest_Sort_KIND_PROGRESS},
			Desc: true,
		},
		Page: &common.Page{Size: 1},
	}
	records := []*models.SuiteRunRecord{
		suiteRun("low", common.Status_STATUS_COMPLETED, deployment.Provider_PROVIDER_DOCKER, 70),
		suiteRun("top", common.Status_STATUS_COMPLETED, deployment.Provider_PROVIDER_DOCKER, 100),
		suiteRun("mid", common.Status_STATUS_COMPLETED, deployment.Provider_PROVIDER_DOCKER, 90),
		suiteRun("wrong-status", common.Status_STATUS_RUNNING, deployment.Provider_PROVIDER_DOCKER, 100),
		suiteRun("wrong-provider", common.Status_STATUS_COMPLETED, deployment.Provider_PROVIDER_YANDEX, 100),
	}

	page, next, err := filterSuiteRunRecords(context.Background(), nil, records, req, "")
	if err != nil {
		t.Fatalf("filter suite runs: %v", err)
	}
	if len(page) != 1 || page[0].GetEntity().GetId() != "top" {
		t.Fatalf("first page = %v, want top", ids(page))
	}
	if next == "" {
		t.Fatal("next page token is empty")
	}

	req.Page.Token = next
	page, next, err = filterSuiteRunRecords(context.Background(), nil, records, req, "")
	if err != nil {
		t.Fatalf("filter suite runs page 2: %v", err)
	}
	if len(page) != 1 || page[0].GetEntity().GetId() != "mid" {
		t.Fatalf("second page = %v, want mid", ids(page))
	}
	if next != "" {
		t.Fatalf("unexpected next token %q", next)
	}
}

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

func TestEnsurePresetSummariesBackfillsDecodedRows(t *testing.T) {
	dbPreset := &models.DatabasePresetRecord{Database: &domain.Database{
		Kind:   domain.Database_KIND_POSTGRES,
		Source: &domain.Database_Params{Params: &domain.DatabaseParams{Version: "16"}},
	}}
	ensureDatabasePresetSummary(dbPreset)
	if dbPreset.GetSummary().GetDbKind() != domain.Database_KIND_POSTGRES || dbPreset.GetSummary().GetVersion() != "16" {
		t.Fatalf("database summary = %+v", dbPreset.GetSummary())
	}

	workloadPreset := &models.WorkloadPresetRecord{Workload: &domain.Workload{
		Protocol:       domain.Workload_PROTOCOL_PG,
		StroppyVersion: "1.2.3",
		Segments: []*domain.Workload_Segment{{
			Name:   "workload",
			Script: "tpcc",
		}},
	}}
	ensureWorkloadPresetSummary(workloadPreset)
	if workloadPreset.GetSummary().GetProtocol() != domain.Workload_PROTOCOL_PG || workloadPreset.GetSummary().GetScript() != "tpcc" {
		t.Fatalf("workload summary = %+v", workloadPreset.GetSummary())
	}

	testPreset := &models.TestPresetRecord{Test: &domain.Test{
		Database: dbPreset.GetDatabase(),
		Workload: workloadPreset.GetWorkload(),
	}}
	ensureTestPresetSummary(testPreset)
	if testPreset.GetSummary().GetDbKind() != domain.Database_KIND_POSTGRES || testPreset.GetSummary().GetStroppyVersion() != "1.2.3" {
		t.Fatalf("test summary = %+v", testPreset.GetSummary())
	}
}

func suiteRun(id string, status common.Status, provider deployment.Provider, progress uint32) *models.SuiteRunRecord {
	return &models.SuiteRunRecord{
		Entity:  entity(id, id),
		Status:  status,
		SuiteId: "suite-1",
		Summary: &models.SuiteRunRecord_Summary{
			Provider:    provider,
			DbKinds:     []domain.Database_Kind{domain.Database_KIND_POSTGRES},
			ProgressPct: progress,
		},
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
