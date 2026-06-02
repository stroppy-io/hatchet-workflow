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

	root.AddCommand(cloudCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
