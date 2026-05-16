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

func templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage test-run templates",
	}
	cmd.AddCommand(templateListCmd())
	cmd.AddCommand(templateGetCmd())
	cmd.AddCommand(templateCreateCmd())
	cmd.AddCommand(templateDeleteCmd())
	cmd.AddCommand(templateInstantiateCmd())
	return cmd
}

func templateListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List test-run templates for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.Template.ListTestRunTemplates(
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

func templateGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a test-run template by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Template.GetTestRunTemplate(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunTemplateId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func templateCreateCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a test-run template",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected")
			}
			resp, err := cli.Template.CreateTestRunTemplate(
				context.Background(),
				connect.NewRequest(&testingpb.CreateTestRunTemplateRequest{
					Template: &testingpb.TestRunTemplate{
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
	cmd.Flags().StringVar(&name, "name", "", "Template name (required)")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func templateDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a test-run template by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Template.DeleteTestRunTemplate(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunTemplateId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func templateInstantiateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "instantiate <template-id>",
		Short: "Instantiate a TestRun from a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.TestRun.InstantiateTestRun(
				context.Background(),
				connect.NewRequest(&testingpb.TestRunTemplateId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
