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
	cmd.AddCommand(presetCmd())
	cmd.AddCommand(packageCmd())
	cmd.AddCommand(settingsCmd())
	cmd.AddCommand(probeCmd())
	cmd.AddCommand(versionCmd())
	cmd.AddCommand(scheduleCmd())
	cmd.AddCommand(templateCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(suiteCmd())
	cmd.AddCommand(compareCmd())
	cmd.AddCommand(webhookCmd())
	cmd.AddCommand(quotaCmd())
	return cmd
}
