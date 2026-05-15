package cmd_cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke current refresh token",
		RunE: func(c *cobra.Command, _ []string) error {
			ctxObj, _ := client.LoadContext()
			if ctxObj == nil || ctxObj.CurrentServer == "" {
				return fmt.Errorf("no active session")
			}
			cred, err := client.LoadCredentials(ctxObj.CurrentServer)
			if err != nil || cred == nil {
				return fmt.Errorf("no saved credentials")
			}
			cli := client.New(ctxObj.CurrentServer, client.WithBearer(cred.AccessToken))
			_, err = cli.Auth.Logout(context.Background(), connect.NewRequest(&iampb.LogoutRequest{RefreshToken: cred.RefreshToken}))
			if err != nil {
				return err
			}
			cred.AccessToken = ""
			cred.RefreshToken = ""
			_ = client.SaveCredentials(cred)
			fmt.Println("ok")
			return nil
		},
	}
}
