package verbs

import (
	"context"
	"testing"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
)

func TestInstallPackage_AptSkipped(t *testing.T) {
	t.Setenv("STROPPY_AGENT_SKIP_APT", "1")

	action := &agentpb.Action{
		Verb: &agentpb.Action_InstallPackage{
			InstallPackage: &agentpb.InstallPackage{
				Package: &catalogpb.Package{
					Source: &catalogpb.Package_PackageSource{
						Source: &catalogpb.Package_PackageSource_Apt{
							Apt: &catalogpb.Package_AptSource{
								AptPackages: []string{"curl"},
							},
						},
					},
				},
			},
		},
	}
	rep := InstallPackage(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED with skip env, got %v; error: %s", rep.Status, rep.Error)
	}
}

func TestInstallPackage_NilPayload(t *testing.T) {
	action := &agentpb.Action{
		Verb: &agentpb.Action_InstallPackage{InstallPackage: nil},
	}
	rep := InstallPackage(context.Background(), action)
	if rep.Status != agentpb.ReportStatus_REPORT_STATUS_FAILED {
		t.Fatalf("expected FAILED for nil payload, got %v", rep.Status)
	}
}
