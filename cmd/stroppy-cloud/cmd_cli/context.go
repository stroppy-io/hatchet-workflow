package cmd_cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func contextCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "context", Short: "Manage local context"}
	use := &cobra.Command{
		Use:   "use --tenant <id>",
		Short: "Switch active tenant",
		RunE: func(c *cobra.Command, _ []string) error {
			tenant, _ := c.Flags().GetString("tenant")
			if tenant == "" {
				return fmt.Errorf("--tenant required")
			}
			ctxObj, _ := client.LoadContext()
			if ctxObj == nil {
				ctxObj = &client.Context{}
			}
			ctxObj.CurrentTenant = tenant
			return client.SaveContext(ctxObj)
		},
	}
	use.Flags().String("tenant", "", "tenant id")
	cmd.AddCommand(use)
	return cmd
}
