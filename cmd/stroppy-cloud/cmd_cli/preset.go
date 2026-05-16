package cmd_cli

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	catalogpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func presetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Manage database and workload presets",
	}
	cmd.AddCommand(presetDatabaseCmd())
	cmd.AddCommand(presetWorkloadCmd())
	return cmd
}

// ── database ──────────────────────────────────────────────────────────────────

func presetDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Database preset operations",
	}
	cmd.AddCommand(presetDatabaseListCmd())
	cmd.AddCommand(presetDatabaseGetCmd())
	cmd.AddCommand(presetDatabaseDeleteCmd())
	cmd.AddCommand(presetDatabaseCloneCmd())
	return cmd
}

func presetDatabaseListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List database presets for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.DatabasePreset.ListDatabasePresets(
				context.Background(),
				connect.NewRequest(&iampb.TenantId{Value: tenantID}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func presetDatabaseGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a database preset by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.DatabasePreset.GetDatabasePreset(
				context.Background(),
				connect.NewRequest(&catalogpb.DatabasePresetId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func presetDatabaseDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a database preset by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.DatabasePreset.DeleteDatabasePreset(
				context.Background(),
				connect.NewRequest(&catalogpb.DatabasePresetId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func presetDatabaseCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <id>",
		Short: "Clone a database preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.DatabasePreset.CloneDatabasePreset(
				context.Background(),
				connect.NewRequest(&catalogpb.DatabasePresetId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

// ── workload ──────────────────────────────────────────────────────────────────

func presetWorkloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Workload preset operations",
	}
	cmd.AddCommand(presetWorkloadListCmd())
	cmd.AddCommand(presetWorkloadGetCmd())
	cmd.AddCommand(presetWorkloadDeleteCmd())
	cmd.AddCommand(presetWorkloadCloneCmd())
	return cmd
}

func presetWorkloadListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workload presets for the current tenant",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, tenantID, err := authedCLI()
			if err != nil {
				return err
			}
			if tenantID == "" {
				return fmt.Errorf("no tenant selected; set CurrentTenant in context")
			}
			resp, err := cli.WorkloadPreset.ListWorkloadPresets(
				context.Background(),
				connect.NewRequest(&iampb.TenantId{Value: tenantID}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func presetWorkloadGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a workload preset by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.WorkloadPreset.GetWorkloadPreset(
				context.Background(),
				connect.NewRequest(&catalogpb.WorkloadPresetId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func presetWorkloadDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a workload preset by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.WorkloadPreset.DeleteWorkloadPreset(
				context.Background(),
				connect.NewRequest(&catalogpb.WorkloadPresetId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func presetWorkloadCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <id>",
		Short: "Clone a workload preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.WorkloadPreset.CloneWorkloadPreset(
				context.Background(),
				connect.NewRequest(&catalogpb.WorkloadPresetId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

// printJSON marshals v to indented JSON and writes it to stdout.
func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
