// Command stroppy-cloud is the single binary for the whole platform: the control
// plane (`serve`), the per-machine worker (`agent`), and the operator CLI (`login`,
// `run`, `bench`, `compare`, `packages`, ...). One cobra tree, one build.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "stroppy-cloud",
		Short:         "Stroppy Cloud — distributed database benchmarking platform",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		serveCmd(),
		agentCmd(),
		newCloudCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
