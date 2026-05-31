package app

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	domain "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/monitor"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run"
	"github.com/stroppy-io/stroppy-cloud/internal/services/test_run_overview"
)

/*
	The record repositories (TestRunRepo, DraftRepo, the preset ports and the
	overview TestRunReader) are now backed by postgres via
	internal/store/gormstore; they are opened and wired in App.Start. This file
	keeps only the pure, non-persistent ports: the Summarizer and the not-yet-backed
	Logs/Metrics readers.
*/

/*
	===== Summarizer (test_run.Summarizer) =====
*/

type summarizer struct{}

var _ test_run.Summarizer = summarizer{}

// Summarize derives the flat Summary projection from a baked spec. Pure, no IO.
func (summarizer) Summarize(spec *domain.TestRun) *models.TestRunRecord_Summary {
	if spec == nil {
		return &models.TestRunRecord_Summary{}
	}
	return &models.TestRunRecord_Summary{
		DbKind:         spec.GetDatabase().GetKind(),
		WorkloadName:   spec.GetWorkload().GetStroppyVersion(),
		StroppyVersion: spec.GetWorkload().GetStroppyVersion(),
		NodeCount:      uint32(len(spec.GetTopology().GetInstances())),
		Provider:       spec.GetProvider().GetProvider(),
	}
}

/*
	===== no-op Logs / Metrics readers (test_run_overview) =====

	The demo serves observability through the Overview engine (Temporal RunState);
	the Logs and Metrics tabs are not backed yet, so these return empty data.
*/

type noopLogReader struct{}

var _ test_run_overview.LogReader = noopLogReader{}

func (noopLogReader) Query(_ context.Context, _ string, _ *api.LogFilter, _ *monitor.LogCursor, _ api.LogScrollDirection, _ uint32) ([]*monitor.LogLine, *monitor.LogCursor, *monitor.LogCursor, error) {
	return nil, nil, nil, nil
}

func (noopLogReader) Stream(_ context.Context, _ string, _ *api.LogFilter, _ *monitor.LogCursor) (<-chan *monitor.LogLine, error) {
	ch := make(chan *monitor.LogLine)
	close(ch)
	return ch, nil
}

func (noopLogReader) Resolve(_ context.Context, _ *monitor.LogRef) (*api.LogFilter, *monitor.LogCursor, error) {
	return &api.LogFilter{}, &monitor.LogCursor{}, nil
}

type noopMetricsReader struct{}

var _ test_run_overview.MetricsReader = noopMetricsReader{}

func (noopMetricsReader) Get(_ context.Context, _ string) (*monitor.RunMetrics, error) {
	return &monitor.RunMetrics{}, nil
}
