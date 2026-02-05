package stroppy

import (
	"context"
	"os"

	"github.com/cenkalti/backoff/v4"
	"github.com/hatchet-dev/hatchet/pkg/client/rest"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/mitchellh/mapstructure"
	"github.com/sourcegraph/conc/pool"
	hatchet_ext "github.com/stroppy-io/hatchet-workflow/internal/core/hatchet-ext"
	"github.com/stroppy-io/hatchet-workflow/internal/core/ids"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/hatchet"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/stroppy"
)

const (
	TestSuiteWorkflowName = "stroppy-test-suite"
	TestSuiteTaskName     = "run-test-suite"
)

func TestSuiteWorkflow(
	c *hatchetLib.Client,
) *hatchetLib.Workflow {
	workflow := c.NewWorkflow(
		TestSuiteWorkflowName,
		hatchetLib.WithWorkflowDescription("Stroppy Test Suite Workflow"),
	)
	workflow.NewTask(
		TestSuiteTaskName,
		hatchet_ext.WTask(func(
			ctx hatchetLib.Context,
			input *hatchet.Workflows_StroppyTestSuite_Input,
		) (*hatchet.Workflows_StroppyTestSuite_Output, error) {
			err := input.Validate()
			if err != nil {
				return nil, err
			}
			inputs := make([]hatchetLib.RunManyOpt, len(input.GetSuite().GetTests()))
			for i, test := range input.GetSuite().GetTests() {
				inputs[i] = hatchetLib.RunManyOpt{
					Opts: []hatchetLib.RunOptFunc{},
					Input: &hatchet.Workflows_StroppyTest_Input{
						Common: &hatchet.Common{
							RunId: ids.NewRunId().String(),
							HatchetServer: &hatchet.HatchetServer{
								Url:   input.GetHatchetUrl(),
								Token: os.Getenv("HATCHET_CLIENT_TOKEN"),
							},
							SupportedCloud: input.GetSupportedCloud(),
						},
						Test: test,
					},
				}
			}
			runRefs, err := workflow.RunMany(ctx, inputs)
			if err != nil {
				return nil, err
			}

			waitPool := pool.NewWithResults[*stroppy.TestResult]().WithContext(ctx).WithCancelOnError().WithFirstError()
			for _, ref := range runRefs {
				waitPool.Go(func(ctx context.Context) (*stroppy.TestResult, error) {
					var run *rest.V1WorkflowRunDetails
					err := backoff.Retry(func() error {
						runModel, err := c.Runs().Get(ctx, ref.RunId)
						if err != nil {
							return err
						}
						run = runModel
						return nil
					}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))
					if err != nil {
						return nil, err
					}
					var result stroppy.TestResult
					err = mapstructure.Decode(run.Run.Output, &result)
					if err != nil {
						return nil, err
					}
					return &result, nil
				})
			}
			results, err := waitPool.Wait()
			if err != nil {
				return nil, err
			}
			ret := make(map[string]*stroppy.TestResult, len(results))
			for _, result := range results {
				ret[result.RunId] = result
			}
			return &hatchet.Workflows_StroppyTestSuite_Output{
				Results: &stroppy.TestSuiteResult{
					Results: ret,
				},
			}, nil
		}),
	)
	return workflow
}
