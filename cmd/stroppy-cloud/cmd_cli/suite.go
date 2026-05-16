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

func suiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suite",
		Short: "Manage test suites",
	}
	cmd.AddCommand(suiteListCmd())
	cmd.AddCommand(suiteGetCmd())
	cmd.AddCommand(suiteCreateCmd())
	cmd.AddCommand(suiteCloneCmd())
	cmd.AddCommand(suiteDeleteCmd())
	cmd.AddCommand(suiteLaunchCmd())
	return cmd
}

func suiteListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List test suites for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.TestSuite.ListTestSuites(
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

func suiteGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a test suite by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestSuite.GetTestSuite(
				context.Background(),
				connect.NewRequest(&testingpb.TestSuiteId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func suiteCreateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a test suite",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected")
			}
			resp, err := cli.TestSuite.CreateTestSuite(
				context.Background(),
				connect.NewRequest(&testingpb.CreateTestSuiteRequest{
					Suite: &testingpb.TestSuite{
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
	cmd.Flags().StringVar(&name, "name", "", "Suite name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func suiteCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <id>",
		Short: "Clone a test suite",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestSuite.CloneTestSuite(
				context.Background(),
				connect.NewRequest(&testingpb.TestSuiteId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func suiteDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a test suite",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestSuite.DeleteTestSuite(
				context.Background(),
				connect.NewRequest(&testingpb.TestSuiteId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func suiteLaunchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "launch <suite-id>",
		Short: "Launch a test suite (creates a TestSuiteRun)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestSuiteRun.LaunchTestSuite(
				context.Background(),
				connect.NewRequest(&testingpb.TestSuiteId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
