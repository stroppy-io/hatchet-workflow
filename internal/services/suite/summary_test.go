package suite

import (
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestScheduleSummaryCountsEnabledCellsAndPreservesRunHistory(t *testing.T) {
	lastRunAt := timestamppb.New(time.Unix(100, 0))
	summary := scheduleSummary(&domain.Suite{
		Cells: []*domain.SuiteCell{
			{Enabled: true},
			{Enabled: false},
			{Enabled: true},
		},
		Schedule: &domain.Schedule{Enabled: true, Cron: "*/5 * * * *"},
	}, &models.SuiteRecord_Summary{
		RunCount:      3,
		LastRunAt:     lastRunAt,
		LastRunStatus: common.Status_STATUS_COMPLETED,
	})

	if got := summary.GetCellCount(); got != 2 {
		t.Fatalf("cell_count = %d, want 2", got)
	}
	if got := summary.GetRunCount(); got != 3 {
		t.Fatalf("run_count = %d, want 3", got)
	}
	if got := summary.GetLastRunStatus(); got != common.Status_STATUS_COMPLETED {
		t.Fatalf("last_run_status = %s, want completed", got)
	}
	if summary.GetLastRunAt() != lastRunAt {
		t.Fatal("last_run_at was not preserved")
	}
}
