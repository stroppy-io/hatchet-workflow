// Command stroppy-cloud is the control-plane binary. `serve` boots the connect
// API + Temporal server worker + agent gateway; `agent` runs the node-side
// Temporal worker reached through that gateway.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "stroppy-cloud",
		Short:         "Database testing orchestrator",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(serveCmd(), agentCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
