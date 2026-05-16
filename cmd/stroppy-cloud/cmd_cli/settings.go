package cmd_cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func settingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Manage tenant settings",
	}
	cmd.AddCommand(settingsListCmd())
	cmd.AddCommand(settingsSetCmd())
	return cmd
}

func settingsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List settings for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.Settings.ListSettings(
				context.Background(),
				connect.NewRequest(&catalogpb.ListSettingsRequest{
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

func settingsSetCmd() *cobra.Command {
	var valueString string
	c := &cobra.Command{
		Use:   "set <id>",
		Short: "Set a setting value by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Settings.SetSetting(
				context.Background(),
				connect.NewRequest(&catalogpb.SetSettingRequest{
					Id: &catalogpb.SettingsItemId{Value: args[0]},
					Value: &catalogpb.SettingsItem_Value{
						Value: &catalogpb.SettingsItem_Value_StringValue{
							StringValue: valueString,
						},
					},
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	c.Flags().StringVar(&valueString, "value-string", "", "string value to set")
	_ = c.MarkFlagRequired("value-string")
	return c
}
