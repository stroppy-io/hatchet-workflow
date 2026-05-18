package cmd_cli

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	stroppypb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy"
	systempb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

// runWaitCmd polls GetTestRun until the underlying DagRun reaches a terminal
// status or the timeout elapses.
func runWaitCmd() *cobra.Command {
	var timeoutS string
	cmd := &cobra.Command{
		Use:   "wait <test-run-id>",
		Short: "Poll a TestRun until its DagRun reaches a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			tout, err := time.ParseDuration(timeoutS)
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), tout)
			defer cancel()
			final, err := waitTerminal(ctx, cli.TestRun, args[0])
			if err != nil {
				return err
			}
			fmt.Println(final)
			return nil
		},
	}
	cmd.Flags().StringVar(&timeoutS, "timeout", "30m", "max time to wait")
	return cmd
}

// runBenchCmd is the mature workflow: instantiate a template, launch it,
// wait for completion, print final status. One-shot replacement for the
// `bench` flow on `main`.
func runBenchCmd() *cobra.Command {
	var templateID string
	var timeoutS string
	cmd := &cobra.Command{
		Use:   "bench --template <id>",
		Short: "Instantiate template → launch → wait for terminal status",
		RunE: func(c *cobra.Command, _ []string) error {
			if templateID == "" {
				return fmt.Errorf("--template required")
			}
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			tout, err := time.ParseDuration(timeoutS)
			if err != nil {
				return fmt.Errorf("--timeout: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), tout)
			defer cancel()

			tr, err := cli.TestRun.InstantiateTestRun(ctx, connect.NewRequest(&testingpb.TestRunTemplateId{Value: templateID}))
			if err != nil {
				return fmt.Errorf("instantiate: %w", err)
			}
			fmt.Println("instantiated:", tr.Msg.GetId().GetValue())

			launched, err := cli.TestRun.LaunchTestRun(ctx, connect.NewRequest(tr.Msg.GetId()))
			if err != nil {
				return fmt.Errorf("launch: %w", err)
			}
			fmt.Println("launched, dag_run_id:", launched.Msg.GetDagRunId().GetValue())

			final, err := waitTerminal(ctx, cli.TestRun, tr.Msg.GetId().GetValue())
			if err != nil {
				return err
			}
			fmt.Println("terminal:", final)
			return nil
		},
	}
	cmd.Flags().StringVar(&templateID, "template", "", "TestRunTemplate id")
	cmd.Flags().StringVar(&timeoutS, "timeout", "30m", "max time to wait")
	return cmd
}

// runDryRunCmd previews the workload config without launching anything.
// Calls StroppyService.PreviewStroppyConfig with the supplied wizard inputs.
func runDryRunCmd() *cobra.Command {
	var script string
	var driver string
	var poolSize uint32
	var version string
	cmd := &cobra.Command{
		Use:   "dry-run",
		Short: "Render the final stroppy-config.json from wizard inputs (no launch)",
		RunE: func(c *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			r, err := cli.Stroppy.PreviewStroppyConfig(context.Background(), connect.NewRequest(&stroppypb.PreviewStroppyConfigRequest{
				StroppyVersion: version,
				Script:         script,
				DriverType:     driver,
				PoolSize:       poolSize,
			}))
			if err != nil {
				return err
			}
			fmt.Println(r.Msg.GetStroppyConfigJson())
			return nil
		},
	}
	cmd.Flags().StringVar(&script, "script", "", "workload script path")
	cmd.Flags().StringVar(&driver, "driver", "postgres", "driver_type")
	cmd.Flags().Uint32Var(&poolSize, "pool-size", 8, "pool size")
	cmd.Flags().StringVar(&version, "version", "", "stroppy version")
	return cmd
}

// waitTerminal polls GetTestRun every 2 s until DagRun status is terminal.
// Returns the final status string.
type testRunClient interface {
	GetTestRun(context.Context, *connect.Request[testingpb.TestRunId]) (*connect.Response[testingpb.GetTestRunResponse], error)
}

func waitTerminal(ctx context.Context, cli testRunClient, id string) (string, error) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		resp, err := cli.GetTestRun(ctx, connect.NewRequest(&testingpb.TestRunId{Value: id}))
		if err != nil {
			return "", err
		}
		st := resp.Msg.GetProgress().GetStatus()
		switch st {
		case systempb.DagRunStatus_DAG_RUN_STATUS_SUCCEEDED,
			systempb.DagRunStatus_DAG_RUN_STATUS_FAILED,
			systempb.DagRunStatus_DAG_RUN_STATUS_CANCELLED:
			return st.String(), nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
		}
	}
}
