package cmd_cli

import (
	"context"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func packageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Manage packages",
	}
	cmd.AddCommand(packageListCmd())
	cmd.AddCommand(packageGetCmd())
	cmd.AddCommand(packageDeleteCmd())
	cmd.AddCommand(packageCloneCmd())
	return cmd
}

func packageListCmd() *cobra.Command {
	var dbKind string
	c := &cobra.Command{
		Use:   "list",
		Short: "List packages",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			req := &catalogpb.ListPackagesRequest{}
			if tenantID != "" {
				req.TenantId = &iampb.TenantId{Value: tenantID}
			}
			if dbKind != "" {
				kind, ok := catalogpb.Database_Kind_value[dbKind]
				if ok {
					k := catalogpb.Database_Kind(kind)
					req.DbKind = &k
				}
			}
			resp, err := cli.Package.ListPackages(
				context.Background(),
				connect.NewRequest(req),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	c.Flags().StringVar(&dbKind, "db-kind", "", "filter by DB kind (e.g. DATABASE_KIND_POSTGRES)")
	return c
}

func packageGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a package by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Package.GetPackage(
				context.Background(),
				connect.NewRequest(&catalogpb.PackageId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func packageDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a package by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Package.DeletePackage(
				context.Background(),
				connect.NewRequest(&catalogpb.PackageId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func packageCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <id>",
		Short: "Clone a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Package.ClonePackage(
				context.Background(),
				connect.NewRequest(&catalogpb.PackageId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
