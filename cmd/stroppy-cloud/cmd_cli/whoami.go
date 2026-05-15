package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func whoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print current user + tenants",
		RunE: func(c *cobra.Command, _ []string) error {
			ctxObj, _ := client.LoadContext()
			if ctxObj == nil {
				return fmt.Errorf("not logged in")
			}
			cred, _ := client.LoadCredentials(ctxObj.CurrentServer)
			if cred == nil {
				return fmt.Errorf("not logged in")
			}
			cli := client.New(ctxObj.CurrentServer, client.WithBearer(cred.AccessToken))
			tenants, err := cli.Tenant.ListMyTenants(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				return err
			}
			out := map[string]any{
				"server":         ctxObj.CurrentServer,
				"tenants":        tenants.Msg.GetTenants(),
				"current_tenant": ctxObj.CurrentTenant,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}
