// Command stroppy-cloud is the product server: SPA, HTTP API, tenants and
// everything that talks to Graphene on their behalf.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/stroppy-io/stroppy-cloud/cmd/stroppy-cloud/application"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stroppy-cloud:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	app, err := application.New(ctx)
	if err != nil {
		return err
	}
	return app.Run(ctx)
}
