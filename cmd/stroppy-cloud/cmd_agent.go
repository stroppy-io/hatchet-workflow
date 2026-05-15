package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func agentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent",
		Short: "Run the remote-host agent (placeholder — implemented in plan 05)",
		RunE: func(c *cobra.Command, _ []string) error {
			return fmt.Errorf("agent mode not implemented yet — see plan 05")
		},
	}
}
