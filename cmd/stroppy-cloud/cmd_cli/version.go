package cmd_cli

import (
	"context"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Stroppy version and commit info",
	}
	cmd.AddCommand(versionListCmd())
	cmd.AddCommand(versionCommitsCmd())
	return cmd
}

func versionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available stroppy versions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Stroppy.ListStroppyVersions(
				context.Background(),
				connect.NewRequest(&emptypb.Empty{}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func versionCommitsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commits",
		Short: "List recent stroppy commits",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Stroppy.ListStroppyCommits(
				context.Background(),
				connect.NewRequest(&emptypb.Empty{}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
