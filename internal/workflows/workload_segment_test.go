package workflows

import (
	"strings"
	"testing"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/domain"
)

func TestWorkloadRunStepIsPerSegment(t *testing.T) {
	component := &deploymentpb.ComponentDeployment{ComponentId: "workload-runner-1"}

	bootstrap := workloadRunStep(component, &domain.Workload_Segment{Name: "Bootstrap"}, 0, false)
	if got, want := bootstrap.GetId(), "900_run_stroppy_bootstrap"; got != want {
		t.Fatalf("segment 0 step id = %q, want %q", got, want)
	}
	if got := bootstrap.GetOrder(); got != 900 {
		t.Fatalf("segment 0 order = %d, want 900", got)
	}
	if script := bootstrap.GetCallCmd().GetSpec().GetScript().GetText(); !strings.Contains(script, "stroppy-config.json") {
		t.Fatalf("segment 0 must run the legacy config name:\n%s", script)
	}

	workloadSeg := workloadRunStep(component, &domain.Workload_Segment{Name: "Workload"}, 1, false)
	if got, want := workloadSeg.GetId(), "910_run_stroppy_workload"; got != want {
		t.Fatalf("segment 1 step id = %q, want %q", got, want)
	}
	if got := workloadSeg.GetOrder(); got != 910 {
		t.Fatalf("segment 1 order = %d, want 910", got)
	}
	if script := workloadSeg.GetCallCmd().GetSpec().GetScript().GetText(); !strings.Contains(script, "stroppy-config-1.json") {
		t.Fatalf("segment 1 must run its indexed config:\n%s", script)
	}
}

func TestSegmentStepSlugSanitizesAndFallsBack(t *testing.T) {
	cases := []struct {
		name  string
		index int
		want  string
	}{
		{"Bootstrap", 0, "bootstrap"},
		{"Load Data", 0, "load_data"},
		{"warm-up!", 0, "warm_up"},
		{"   ", 3, "seg3"},
		{"", 2, "seg2"},
	}
	for _, tc := range cases {
		if got := segmentStepSlug(&domain.Workload_Segment{Name: tc.name}, tc.index); got != tc.want {
			t.Fatalf("slug(%q, %d) = %q, want %q", tc.name, tc.index, got, tc.want)
		}
	}
}
