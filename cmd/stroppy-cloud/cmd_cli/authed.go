package cmd_cli

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
)

// authedCLI loads the current server + credentials and returns a Client with
// bearer token wired, plus the current tenant id.
func authedCLI() (*client.Client, string, error) {
	ctx, err := client.LoadContext()
	if err != nil || ctx == nil || ctx.CurrentServer == "" {
		return nil, "", fmt.Errorf("not logged in (use `stroppy-cloud login`)")
	}
	cred, err := client.LoadCredentials(ctx.CurrentServer)
	if err != nil || cred == nil {
		return nil, "", fmt.Errorf("no credentials for %s", ctx.CurrentServer)
	}
	cli := client.New(ctx.CurrentServer, client.WithBearer(cred.AccessToken))
	return cli, ctx.CurrentTenant, nil
}
