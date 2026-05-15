package cmd_cli

import "github.com/spf13/cobra"

// Root returns the parent command for all client subcommands.
func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Client subcommands (login, run, suite, etc.)",
	}
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(whoamiCmd())
	cmd.AddCommand(contextCmd())
	return cmd
}
