package cmd_cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

func quotaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Manage resource quotas",
	}
	cmd.AddCommand(quotaListCmd())
	return cmd
}

func quotaListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List quotas for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.Quota.GetQuotas(
				context.Background(),
				connect.NewRequest(&opspb.GetQuotasRequest{
					TenantId: &iampb.TenantId{Value: tenantID},
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
