package test_run

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// cloneSpec deep-copies a baked spec so mutating the new run's id never aliases
// the caller's request message (or the source record when re-running).
func cloneSpec(spec *domain.TestRun) *domain.TestRun {
	return proto.Clone(spec).(*domain.TestRun)
}

// ratingOrDefault resolves an optional bool flag to its value, or the documented
// server default when unset (in_tenant_rating -> true, in_global_rating -> false).
func ratingOrDefault(v *bool, def bool) bool {
	if v != nil {
		return *v
	}
	return def
}

// touchUpdated bumps the record's updated_at, allocating Timings if absent.
func touchUpdated(rec *models.TestRunRecord, now *timestamppb.Timestamp) {
	if rec.Entity == nil {
		rec.Entity = &commonpb.Entity{}
	}
	if rec.Entity.Timings == nil {
		rec.Entity.Timings = &commonpb.Timings{CreatedAt: now}
	}
	rec.Entity.Timings.UpdatedAt = now
}

func markDeleted(rec *models.TestRunRecord, now *timestamppb.Timestamp) {
	touchUpdated(rec, now)
	rec.Entity.Timings.DeletedAt = now
}

func markCancelled(rec *models.TestRunRecord, now *timestamppb.Timestamp) {
	rec.Status = commonpb.Status_STATUS_CANCELLED
	if rec.Summary == nil {
		rec.Summary = &models.TestRunRecord_Summary{}
	}
	if rec.Summary.StartedAt == nil {
		rec.Summary.StartedAt = now
	}
	if rec.Summary.FinishedAt == nil {
		rec.Summary.FinishedAt = now
	}
	if start := rec.Summary.GetStartedAt(); start != nil && now != nil {
		d := now.AsTime().Sub(start.AsTime())
		if d < 0 {
			d = 0
		}
		rec.Summary.Duration = durationpb.New(d)
	}
	touchUpdated(rec, now)
}

func markFailed(rec *models.TestRunRecord, now *timestamppb.Timestamp) {
	rec.Status = commonpb.Status_STATUS_FAILED
	if rec.Summary == nil {
		rec.Summary = &models.TestRunRecord_Summary{}
	}
	if rec.Summary.StartedAt == nil {
		rec.Summary.StartedAt = now
	}
	if rec.Summary.FinishedAt == nil {
		rec.Summary.FinishedAt = now
	}
	if start := rec.Summary.GetStartedAt(); start != nil && now != nil {
		d := now.AsTime().Sub(start.AsTime())
		if d < 0 {
			d = 0
		}
		rec.Summary.Duration = durationpb.New(d)
	}
	touchUpdated(rec, now)
}

// specName derives a human label for the run record from the baked workload
// version, falling back to a generic label.
func specName(spec *domain.TestRun) string {
	if v := spec.GetWorkload().GetStroppyVersion(); v != "" {
		return "stroppy " + v
	}
	return "test run"
}

// derivePresetName builds a default preset name from the source run's summary.
func derivePresetName(rec *models.TestRunRecord) string {
	if n := rec.GetEntity().GetName(); n != "" {
		return n + " preset"
	}
	return "preset from run " + rec.GetEntity().GetId()
}
