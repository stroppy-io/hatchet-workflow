package workflows

import (
	"context"
	"fmt"
	"strings"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/oklog/ulid/v2"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/workflows/provision"
)

type QuotaManager interface {
	ReserveQuotas(ctx context.Context, quotas []*crossplane.Quota) error
}

type IpManager interface {
	GetAvailableIp(ctx context.Context) (*crossplane.Ip, error)
}

func NightlyCloudStroppyFn(
	c *hatchetLib.Client,
	quotaManager QuotaManager,
	ipManager IpManager,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		"nightly-cloud-stroppy",
		hatchetLib.WithWorkflowDescription("Nightly Cloud Stroppy Workflow"),
	)

	//workflow.OnFailure(func(ctx hatchetLib.Context, input domain.FailureInput) (domain.FailureHandlerOutput, error) {
	//	if input.ShouldFail{
	//
	//	}
	//})

	runId := strings.ToLower(ulid.Make().String())
	provisioner := provision.New()

	provisionTask := workflow.NewTask(
		"provision",
		func(ctx hatchetLib.Context, input *hatchet.NightlyCloudStroppyRequest) (*hatchet.NightlyCloudStroppyResponse, error) {
			if input.GetCloud() != crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX {
				return nil, fmt.Errorf("unsupported cloud: %s", input.GetCloud())
			}
			neededDeployments := make(map[string]*crossplane.Deployment)
			vanillaId := fmt.Sprintf("vanilla-postgres-vm-%s", runId)
			neededDeployments["vanilla-postgres-vm"] = &crossplane.Deployment{
				Id:             vanillaId,
				SupportedCloud: crossplane.SupportedCloud_SUPPORTED_CLOUD_YANDEX,
				Deployment: &crossplane.Deployment_Vm_{
					Vm: &crossplane.Deployment_Vm{
						MachineInfo: input.GetPostgresVm(),
						CloudInit:   nil,
						NetworkParams: &crossplane.Deployment_Vm_NetworkParams{
							InternalIp:   nil,
							AssignedCidr: nil,
							PublicIp:     nil,
						},
					},
				},
			}
			neededDeployments["stroppy"] = input.GetStroppyVm()
			resp, err := provisioner.ProvisionCloud(ctx, &hatchet.ProvisionCloudRequest{
				RunId:       runId,
				Deployments: input.GetDeployments(),
			})
			if err != nil {
				return nil, err
			}
			return &hatchet.NightlyCloudStroppyResponse{
				RunId:       resp.GetRunId(),
				Deployments: resp.GetDeployments(),
				GrafanaUrl:  "",
			}, nil
		},
	)

	//removeCloudTask := workflow.NewTask(
	//	"provision-remove",
	//	provisioner.RemoveCloud,
	//	hatchetLib.WithParents(provisionTask),
	//)

	return workflow
	//return c.Tas("first-workflow", func(ctx hatchet.Context, input SimpleInput) (SimpleOutput, error) {
	//
	//	return SimpleOutput{
	//		TransformedMessage: strings.ToLower(input.Message),
	//	}, nil
	//})
}
