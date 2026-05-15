package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/stroppy-io/stroppy-cloud/cmd/stroppy-cloud/cmd_cli"
)

func main() {
	root := &cobra.Command{
		Use:   "stroppy-cloud",
		Short: "Stroppy Cloud control plane",
	}
	root.AddCommand(serverCmd())
	root.AddCommand(agentCmd())
	root.AddCommand(cmd_cli.Root())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
