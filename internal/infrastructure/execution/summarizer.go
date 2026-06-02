package execution

import (
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	models "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
)

// RunSummarizer implements test_run.Summarizer. It derives the flat, queryable
// Summary projection from a baked domain.TestRun spec. Pure: no IO, no clock.
type RunSummarizer struct{}

var _ test_run.Summarizer = RunSummarizer{}

// NewRunSummarizer builds the test_run.Summarizer adapter.
func NewRunSummarizer() RunSummarizer { return RunSummarizer{} }

// Summarize reads the baked spec's oneofs and sub-messages and flattens them into
// the denormalized Summary the list/overview UIs query against. Reading from the
// spec shape of THIS repo: the db kind comes from the database, the workload name
// and stroppy version from the workload, node_count from the generated topology
// spec, and the provider from the infrastructure plan.
func (RunSummarizer) Summarize(spec *domain.TestRun) *models.TestRunRecord_Summary {
	if spec == nil {
		return &models.TestRunRecord_Summary{}
	}

	db := spec.GetDatabase()
	wl := spec.GetWorkload()

	summary := &models.TestRunRecord_Summary{
		DbKind:           db.GetKind(),
		WorkloadName:     workloadName(wl),
		StroppyVersion:   wl.GetStroppyVersion(),
		WorkloadProtocol: wl.GetProtocol(),
		NodeCount:        uint32(len(spec.GetTopologySpec().GetNodes())),
		Provider:         spec.GetInfrastructurePlan().GetProvider(),
		TopologyLabel:    topologyLabel(spec),
	}
	// Carry the db preset id when the database was sourced from a preset.
	if pid := db.GetDatabasePresetId(); pid != nil {
		summary.DbPresetId = pid.GetId()
	}
	return summary
}

// workloadName derives a human label for the workload. The baked workload carries
// no explicit name, so fall back to the stroppy version (the most descriptive
// stable field), then to a generic label.
func workloadName(wl *domain.Workload) string {
	if wl == nil {
		return ""
	}
	if v := wl.GetStroppyVersion(); v != "" {
		return "stroppy " + v
	}
	return ""
}

// topologyLabel returns the generated topology's "label" annotation when present,
// else an empty string (the UI falls back to node_count + db kind).
func topologyLabel(spec *domain.TestRun) string {
	labels := spec.GetTopologySpec().GetLabels()
	if labels == nil {
		return ""
	}
	return labels["label"]
}
