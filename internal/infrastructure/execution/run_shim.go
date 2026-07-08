package execution

// Package doc: this file exists ONLY for the SP-E Task 3 -> Task 4 gap (see
// the SP-E plan's Task 3 "Sequencing note"). Task 3 flips RunRecipeWorkflow's
// write path from models.TestRunRecord/test_run_records onto models.Run/
// run_records (see run_persistence.go), but OverviewReader (overview.go)
// still reads a *models.TestRunRecord via SnapshotRunReader — it is not
// migrated until Task 4. Without an adapter, a run started AFTER this task
// lands would be invisible to Overview (its row lives in run_records, but
// SnapshotRunReader only ever looked in test_run_records), degrading the
// Topology/Agents tabs to "not found" instead of rendering.
//
// RunToTestRunRecord below is a small, deliberately LOSSY *models.Run ->
// *models.TestRunRecord adapter: it populates only the fields overview.go's
// existing, unmigrated functions actually read off a fresh recipe run's
// record (Entity/Status/Summary/RuntimeState — see overview.go's
// overviewFromRecord/topologyFromRecordWithRunState/workersFromRecord, which
// key off RuntimeState and Summary, not Spec/InfrastructureState/
// DeploymentPlan, for a recipe run already). It intentionally leaves
// Spec/InfrastructureState/DeploymentPlan/RecipeTopology nil — a classic
// domain.TestRun spec never existed for a recipe run in the first place (see
// TestRunRecord's own doc comment), and RecipeTopology's classic-shaped
// projection is superseded by Run.Topology, which overview.go does not yet
// know how to read (that wiring is Task 4's job).
//
// internal/app/glue.go's snapshotRunReader is the actual call site: it tries
// test_run_records first (pre-cutover / historical runs), then falls back to
// run_records + this shim for a run that only exists there (post-cutover).
// DELETE this file once Task 4 lands (SnapshotRunReader/OverviewReader read
// *models.Run natively and no longer need a TestRunRecord-shaped adapter).

import (
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// RunToTestRunRecord adapts run onto the TestRunRecord shape SnapshotRunReader
// (overview.go) still expects, for the Task 3 -> Task 4 transition window
// only — see this file's package doc. Returns nil for a nil run.
func RunToTestRunRecord(run *models.Run) *models.TestRunRecord {
	if run == nil {
		return nil
	}
	return &models.TestRunRecord{
		Entity:         run.GetEntity(),
		Status:         run.GetStatus(),
		Trigger:        run.GetTrigger(),
		InTenantRating: run.GetInTenantRating(),
		InGlobalRating: run.GetInGlobalRating(),
		Summary:        runSummaryToTestRunSummary(run.GetSummary()),
		RuntimeState:   run.GetRuntimeState(),
		RecipeId:       run.GetWorkflowId(),
	}
}

// runSummaryToTestRunSummary field-copies a models.Run_Summary onto a
// models.TestRunRecord_Summary — the two messages share an identical field
// set (see models/test_run.proto's Run.Summary doc: "same 15 fields as
// TestRunRecord.Summary"), so this is a straight, lossless copy (unlike
// RunToTestRunRecord's own top-level lossiness). Returns nil for nil.
func runSummaryToTestRunSummary(s *models.Run_Summary) *models.TestRunRecord_Summary {
	if s == nil {
		return nil
	}
	return &models.TestRunRecord_Summary{
		DbKind:           s.GetDbKind(),
		DbPresetId:       s.GetDbPresetId(),
		DbPresetName:     s.GetDbPresetName(),
		WorkloadPresetId: s.GetWorkloadPresetId(),
		WorkloadName:     s.GetWorkloadName(),
		StroppyVersion:   s.GetStroppyVersion(),
		WorkloadProtocol: s.GetWorkloadProtocol(),
		TestPresetId:     s.GetTestPresetId(),
		TestPresetName:   s.GetTestPresetName(),
		TopologyLabel:    s.GetTopologyLabel(),
		NodeCount:        s.GetNodeCount(),
		Provider:         s.GetProvider(),
		ProgressPct:      s.GetProgressPct(),
		StartedAt:        s.GetStartedAt(),
		FinishedAt:       s.GetFinishedAt(),
		Duration:         s.GetDuration(),
	}
}
