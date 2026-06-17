package workload

import "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"

// PrimarySegment returns the first segment of a workload, or nil when the
// workload carries none. Renderers that still assume a single stroppy
// invocation (sizing, topology labels, preset summaries) read the primary
// segment; the multi-segment deployment/workflow path iterates GetSegments
// directly.
func PrimarySegment(w *domain.Workload) *domain.Workload_Segment {
	segments := w.GetSegments()
	if len(segments) == 0 {
		return nil
	}
	return segments[0]
}
