package cmd_cli

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"

	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin"
	agentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent"
	commonpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	opspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops"
)

// adminCmd is the root subcommand for platform-admin operations.
func adminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Platform-admin operations (tenants, users, artifacts)",
	}
	// Tenant subcommands
	cmd.AddCommand(adminTenantListCmd())
	cmd.AddCommand(adminTenantCreateCmd())
	cmd.AddCommand(adminTenantDeleteCmd())
	// User subcommands
	cmd.AddCommand(adminUserListCmd())
	cmd.AddCommand(adminUserCreateCmd())
	cmd.AddCommand(adminUserDeleteCmd())
	cmd.AddCommand(adminUserResetPasswordCmd())
	// Binary artifact subcommands
	cmd.AddCommand(adminArtifactListCmd())
	cmd.AddCommand(adminArtifactPrewarmCmd())
	cmd.AddCommand(adminArtifactEvictCmd())
	cmd.AddCommand(adminArtifactGetCmd())
	return cmd
}

// ─── Tenant ──────────────────────────────────────────────────────────────────

func adminTenantListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tenant-list",
		Short: "List all tenants (platform-admin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Admin.ListAllTenants(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func adminTenantCreateCmd() *cobra.Command {
	var name, description, ownerUserID string
	cmd := &cobra.Command{
		Use:   "tenant-create",
		Short: "Create a tenant with an optional owner (platform-admin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			t := &iampb.Tenant{Identity: &commonpb.Identity{Name: name}}
			if description != "" {
				t.Identity.Description = &description
			}
			req := &adminpb.AdminCreateTenantRequest{
				Tenant: t,
			}
			if ownerUserID != "" {
				req.OwnerUserId = &iampb.UserId{Value: ownerUserID}
			}
			resp, err := cli.Admin.CreateTenant(context.Background(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "tenant name (required)")
	cmd.Flags().StringVar(&description, "description", "", "tenant description")
	cmd.Flags().StringVar(&ownerUserID, "owner-user-id", "", "user ID to assign as OWNER member")
	return cmd
}

func adminTenantDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tenant-delete <id>",
		Short: "Hard-delete a tenant and all its members (platform-admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Admin.DeleteTenantHard(
				context.Background(),
				connect.NewRequest(&iampb.TenantId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

// ─── User ─────────────────────────────────────────────────────────────────────

func adminUserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "user-list",
		Short: "List all users across all tenants (platform-admin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Admin.ListAllUsers(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func adminUserCreateCmd() *cobra.Command {
	var email, nickname, password string
	cmd := &cobra.Command{
		Use:   "user-create",
		Short: "Create a user (platform-admin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if email == "" || password == "" {
				return fmt.Errorf("--email and --password are required")
			}
			if nickname == "" {
				// Default nickname from email local-part.
				parts := strings.SplitN(email, "@", 2)
				nickname = parts[0]
			}
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Admin.CreateUser(
				context.Background(),
				connect.NewRequest(&adminpb.AdminCreateUserRequest{
					User:     &iampb.User{Email: email, Nickname: nickname},
					Password: password,
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "user email (required)")
	cmd.Flags().StringVar(&nickname, "nickname", "", "display nickname (defaults to email local-part)")
	cmd.Flags().StringVar(&password, "password", "", "initial password (required)")
	return cmd
}

func adminUserDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "user-delete <id>",
		Short: "Hard-delete a user (platform-admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Admin.DeleteUser(
				context.Background(),
				connect.NewRequest(&iampb.UserId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func adminUserResetPasswordCmd() *cobra.Command {
	var newPassword string
	cmd := &cobra.Command{
		Use:   "user-reset-password <id>",
		Short: "Reset a user's password and revoke all sessions (platform-admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if newPassword == "" {
				return fmt.Errorf("--new-password is required")
			}
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.Admin.ResetUserPassword(
				context.Background(),
				connect.NewRequest(&adminpb.AdminResetPasswordRequest{
					UserId:      &iampb.UserId{Value: args[0]},
					NewPassword: newPassword,
				}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&newPassword, "new-password", "", "new password (required)")
	return cmd
}

// ─── Binary Artifact ─────────────────────────────────────────────────────────

func adminArtifactListCmd() *cobra.Command {
	var namePrefix string
	cmd := &cobra.Command{
		Use:   "artifact-list",
		Short: "List binary artifacts in cache (platform-admin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.BinaryCacheAdmin.ListArtifacts(
				context.Background(),
				connect.NewRequest(&adminpb.ListArtifactsRequest{NamePrefix: namePrefix}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringVar(&namePrefix, "name-prefix", "", "filter by name prefix")
	return cmd
}

func adminArtifactPrewarmCmd() *cobra.Command {
	var items []string
	cmd := &cobra.Command{
		Use:   "artifact-prewarm",
		Short: "Prewarm binary artifact cache entries (platform-admin)",
		Long: `Prewarm accepts a list of name:version:filename triples separated by commas.
Example: --items stroppy:v4.0.0:stroppy-linux-amd64,node_exporter:1.8.0:node_exporter-linux-amd64`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(items) == 0 {
				return fmt.Errorf("--items is required (name:version:filename,...)")
			}
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			req := &adminpb.PrewarmRequest{}
			for _, item := range items {
				parts := strings.SplitN(item, ":", 3)
				if len(parts) != 3 {
					return fmt.Errorf("invalid item %q: expected name:version:filename", item)
				}
				req.Items = append(req.Items, &agentpb.ResolveArtifactRequest{
					Name:     parts[0],
					Version:  parts[1],
					Filename: parts[2],
				})
			}
			resp, err := cli.BinaryCacheAdmin.Prewarm(
				context.Background(),
				connect.NewRequest(req),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
	cmd.Flags().StringSliceVar(&items, "items", nil, "name:version:filename triples (repeatable or comma-separated)")
	return cmd
}

func adminArtifactEvictCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "artifact-evict <id>",
		Short: "Soft-delete a binary artifact from cache (platform-admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.BinaryCacheAdmin.EvictArtifact(
				context.Background(),
				connect.NewRequest(&opspb.BinaryArtifactId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}

func adminArtifactGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "artifact-get <id>",
		Short: "Get a binary artifact by ID (platform-admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := authedCLI()
			if err != nil {
				return err
			}
			resp, err := cli.BinaryCacheAdmin.GetArtifact(
				context.Background(),
				connect.NewRequest(&opspb.BinaryArtifactId{Value: args[0]}),
			)
			if err != nil {
				return err
			}
			return printJSON(resp.Msg)
		},
	}
}
