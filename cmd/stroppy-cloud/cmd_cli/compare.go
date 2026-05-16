package cmd_cli

import (
	"context"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func compareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare test run metrics",
	}
	cmd.AddCommand(compareRunsCmd())
	return cmd
}

func compareRunsCmd() *cobra.Command {
	var metrics []string
	cmd := &cobra.Command{
		Use:   "runs <run-a-id> <run-b-id>",
		Short: "Compare metrics between two test runs",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Comparison.CompareRuns(
				context.Background(),
				connect.NewRequest(&testingpb.CompareRunsRequest{
					A:           &testingpb.TestRunId{Value: args[0]},
					B:           &testingpb.TestRunId{Value: args[1]},
					MetricNames: metrics,
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringSliceVar(&metrics, "metric", nil, "Metric names to compare (repeatable; default: all)")
	return cmd
}
