package stroppy

import (
	"context"
	"fmt"
	"time"

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
) *hatchetLib.StandaloneTask {
	return c.NewStandaloneTask(
		TestSuiteWorkflowName,
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
							RunId:          ids.NewUlid().Lower().String(),
							HatchetServer:  input.GetHatchetServer(),
							SupportedCloud: input.GetSupportedCloud(),
						},
						Test: test,
					},
				}
			}
			runRefs, err := c.RunMany(ctx, TestRunWorkflowName, inputs)
			if err != nil {
				return nil, err
			}

			waitPool := pool.NewWithResults[*stroppy.TestResult]().WithContext(ctx).WithCancelOnError().WithFirstError()
			for _, ref := range runRefs {
				waitPool.Go(func(ctx context.Context) (*stroppy.TestResult, error) {
					run, err := waitForRunCompletion(ctx, c, ref.RunId)
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
		hatchetLib.WithWorkflowDescription("Stroppy Test Suite Workflow"),
	)
}

func waitForRunCompletion(ctx context.Context, c *hatchetLib.Client, runID string) (*rest.V1WorkflowRunDetails, error) {
	backoffCfg := backoff.NewExponentialBackOff()
	backoffCfg.InitialInterval = 500 * time.Millisecond
	backoffCfg.MaxInterval = 5 * time.Second
	backoffCfg.MaxElapsedTime = 0 // rely on ctx for cancellation

	var run *rest.V1WorkflowRunDetails
	err := backoff.Retry(func() error {
		runModel, err := c.Runs().Get(ctx, runID)
		if err != nil {
			return err
		}
		run = runModel

		switch run.Run.Status {
		case rest.V1TaskStatusCOMPLETED:
			return nil
		case rest.V1TaskStatusFAILED, rest.V1TaskStatusCANCELLED:
			msg := ""
			if run.Run.ErrorMessage != nil {
				msg = *run.Run.ErrorMessage
			}
			return backoff.Permanent(fmt.Errorf("workflow %s finished with status %s: %s", runID, run.Run.Status, msg))
		default:
			return fmt.Errorf("workflow %s not finished (status %s)", runID, run.Run.Status)
		}
	}, backoff.WithContext(backoffCfg, ctx))
	if err != nil {
		return nil, err
	}
	return run, nil
}
