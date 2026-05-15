package cmd_cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

func loginCmd() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "login --server <url>",
		Short: "Authenticate and persist tokens",
		RunE: func(c *cobra.Command, _ []string) error {
			email, pwd, err := readCreds()
			if err != nil {
				return err
			}
			cli := client.New(server)
			resp, err := cli.Auth.Login(context.Background(), connect.NewRequest(&iampb.LoginRequest{Email: email, Password: pwd}))
			if err != nil {
				return err
			}
			pair := resp.Msg.GetTokens()
			cred := &client.Credentials{
				Server:       server,
				AccessToken:  pair.GetAccessToken(),
				RefreshToken: pair.GetRefreshToken(),
				ExpiresAt:    time.Now().Add(pair.GetAccessTokenExpiresIn().AsDuration()),
			}
			if err := client.SaveCredentials(cred); err != nil {
				return err
			}
			ctxObj, _ := client.LoadContext()
			if ctxObj == nil {
				ctxObj = &client.Context{}
			}
			ctxObj.CurrentServer = server
			_ = client.SaveContext(ctxObj)
			fmt.Println("ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&server, "server", "http://localhost:8080", "server URL")
	return cmd
}

func readCreds() (string, string, error) {
	r := bufio.NewReader(os.Stdin)
	fmt.Print("email: ")
	email, err := r.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	fmt.Print("password: ")
	pwdBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return "", "", err
	}
	email = trimLine(email)
	return email, string(pwdBytes), nil
}

func trimLine(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
