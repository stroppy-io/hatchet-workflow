package cmd_cli

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

func webhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage webhooks",
	}
	cmd.AddCommand(webhookListCmd())
	cmd.AddCommand(webhookGetCmd())
	cmd.AddCommand(webhookCreateCmd())
	cmd.AddCommand(webhookDeleteCmd())
	cmd.AddCommand(webhookTestCmd())
	return cmd
}

func webhookListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List webhooks for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.Webhook.ListWebhooks(
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

func webhookGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a webhook by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Webhook.GetWebhook(
				context.Background(),
				connect.NewRequest(&opspb.WebhookId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func webhookCreateCmd() *cobra.Command {
	var url, name, events string
	var enabled bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a webhook",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			if url == "" {
				return fmt.Errorf("--url is required")
			}

			var eventList []opspb.WebhookEvent
			if events != "" {
				for _, e := range strings.Split(events, ",") {
					e = strings.TrimSpace(e)
					v, ok := opspb.WebhookEvent_value[e]
					if !ok {
						return fmt.Errorf("unknown event: %s", e)
					}
					eventList = append(eventList, opspb.WebhookEvent(v))
				}
			}

			w := &opspb.Webhook{
				Url:     url,
				Enabled: enabled,
				Events:  eventList,
			}
			if name != "" {
				w.Identity = &commonpb.Identity{Name: name}
			}

			resp, err := cli.Webhook.CreateWebhook(
				context.Background(),
				connect.NewRequest(&opspb.CreateWebhookRequest{Webhook: w}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "target URL (required)")
	cmd.Flags().StringVar(&name, "name", "", "display name")
	cmd.Flags().StringVar(&events, "events", "", "comma-separated event names (default: all)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable webhook")
	return cmd
}

func webhookDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Webhook.DeleteWebhook(
				context.Background(),
				connect.NewRequest(&opspb.WebhookId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func webhookTestCmd() *cobra.Command {
	var event string
	cmd := &cobra.Command{
		Use:   "test <id>",
		Short: "Send a test ping to a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			req := &opspb.TestWebhookRequest{
				Id: &opspb.WebhookId{Value: args[0]},
			}
			if event != "" {
				v, ok := opspb.WebhookEvent_value[event]
				if !ok {
					return fmt.Errorf("unknown event: %s", event)
				}
				req.Event = opspb.WebhookEvent(v)
			}
			resp, err := cli.Webhook.TestWebhook(
				context.Background(),
				connect.NewRequest(req),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "event name to test (default: WEBHOOK_EVENT_RUN_STARTED)")
	return cmd
}
