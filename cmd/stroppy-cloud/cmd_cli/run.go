package cmd_cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	testingpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing"
)

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Manage test runs",
	}
	cmd.AddCommand(runListCmd())
	cmd.AddCommand(runGetCmd())
	cmd.AddCommand(runCreateCmd())
	cmd.AddCommand(runLaunchCmd())
	cmd.AddCommand(runDeleteCmd())
	cmd.AddCommand(runCancelCmd())
	return cmd
}

func runListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List test runs for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.TestRun.ListTestRuns(
				context.Background(),
				connect.NewRequest(&iampb.TenantId{Value: tenantID}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func runGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a test run by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestRun.GetTestRun(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func runCreateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a test run",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected")
			}
			resp, err := cli.TestRun.CreateTestRun(
				context.Background(),
				connect.NewRequest(&testingpb.CreateTestRunRequest{
					TestRun: &testingpb.TestRun{
						TenantId: &iampb.TenantId{Value: tenantID},
						Identity: &commonpb.Identity{Name: name},
					},
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Run name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func runLaunchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "launch <id>",
		Short: "Launch a test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestRun.LaunchTestRun(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func runDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestRun.DeleteTestRun(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func runCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestRun.CancelTestRun(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
