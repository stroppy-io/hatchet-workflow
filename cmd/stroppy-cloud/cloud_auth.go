package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/protobuf/types/known/emptypb"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// prompt reads a trimmed line from stdin after printing msg.
func prompt(msg string) string {
	fmt.Print(msg)
	s := bufio.NewScanner(os.Stdin)
	s.Scan()
	return strings.TrimSpace(s.Text())
}

func cloudLoginCmd() *cobra.Command {
	var username, password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate (AuthService.Login) and store tokens for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve server (flag, env, prompt).
			server := cloudServer
			if server == "" {
				server = os.Getenv("STROPPY_SERVER")
			}
			if server == "" {
				server = prompt("Server address [" + defaultServer + "]: ")
				if server == "" {
					server = defaultServer
				}
			}

			if username == "" {
				username = prompt("Username / email: ")
			}
			if password == "" {
				fmt.Print("Password: ")
				pw, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = string(pw)
			}

			conn, err := dialAnon(server)
			if err != nil {
				return fmt.Errorf("dial %s: %w", server, err)
			}
			defer conn.Close()

			ctx, cancel := callCtx()
			defer cancel()

			authClient := uipb.NewAuthServiceClient(conn)
			loginResp, err := authClient.Login(ctx, &uipb.LoginRequest{
				Email:    username,
				Password: password,
			})
			if err != nil {
				return fmt.Errorf("login failed: %w", err)
			}
			tokens := loginResp.GetTokens()
			accessToken := tokens.GetAccessToken()
			refreshToken := tokens.GetRefreshToken()
			if accessToken == "" {
				return fmt.Errorf("login returned no access token")
			}

			// Identify the account (Me) with the fresh token.
			authedConn, err := dialAnonWithToken(server, accessToken)
			if err != nil {
				return fmt.Errorf("dial %s: %w", server, err)
			}
			defer authedConn.Close()
			authed := uipb.NewAuthServiceClient(authedConn)

			me, err := authed.Me(ctx, &emptypb.Empty{})
			if err != nil {
				return fmt.Errorf("identify account (Me): %w", err)
			}

			// Enumerate tenants. There is no SelectTenant RPC: tenant selection
			// is client-side and carried as tenant_id on each later request.
			tenantClient := uipb.NewTenantServiceClient(authedConn)
			tenantList, err := tenantClient.ListMyTenants(ctx, &emptypb.Empty{})
			if err != nil {
				return fmt.Errorf("list tenants: %w", err)
			}
			tenants := tenantList.GetTenants()

			selectedTenant := cloudTenant
			switch {
			case selectedTenant != "":
				// explicit --tenant wins
			case len(tenants) == 1:
				selectedTenant = tenants[0].GetEntity().GetId().GetValue()
			case len(tenants) > 1:
				fmt.Println("Available tenants:")
				for i, t := range tenants {
					fmt.Printf("  [%d] %s\n", i+1, tenantLabel(t))
				}
				choice := prompt("Select tenant [1]: ")
				idx := 0
				if choice != "" {
					n, err := strconv.Atoi(choice)
					if err != nil || n < 1 || n > len(tenants) {
						return fmt.Errorf("invalid tenant selection: %s", choice)
					}
					idx = n - 1
				}
				selectedTenant = tenants[idx].GetEntity().GetId().GetValue()
			}

			// Persist into the active profile.
			creds, err := loadCredentials()
			if err != nil {
				return fmt.Errorf("load credentials: %w", err)
			}
			pname := resolveProfile(creds)
			creds.Profiles[pname] = &Profile{
				Server:       strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(server, "http://"), "https://"), "/"),
				Tenant:       selectedTenant,
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			}
			creds.Current = pname
			if err := creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}

			who := me.GetEmail()
			if who == "" {
				who = me.GetNickname()
			}
			fmt.Printf("Logged in as %s\n", who)
			if selectedTenant != "" {
				fmt.Printf("Tenant:  %s\n", selectedTenant)
			} else {
				fmt.Println("Tenant:  (none — pass --tenant or run: stroppy-cloud cloud use <tenant-id>)")
			}
			fmt.Printf("Profile: %s | saved to %s\n", pname, credentialsPath())
			return nil
		},
	}

	cmd.Flags().StringVar(&username, "username", "", "login username or email")
	cmd.Flags().StringVar(&password, "password", "", "login password (omit for interactive prompt)")
	return cmd
}

func cloudLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the session (AuthService.Logout) and drop the local profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}
			pname := resolveProfile(creds)
			p := creds.Profiles[pname]
			if p != nil && p.AccessToken != "" {
				server := resolveServer(p)
				if conn, derr := dialAnonWithToken(server, p.AccessToken); derr == nil {
					ctx, cancel := callCtx()
					client := uipb.NewAuthServiceClient(conn)
					_, _ = client.Logout(ctx, &uipb.LogoutRequest{RefreshToken: p.RefreshToken})
					cancel()
					conn.Close()
				}
			}

			delete(creds.Profiles, pname)
			if creds.Current == pname {
				creds.Current = "default"
			}
			if err := creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
			fmt.Printf("Logged out (profile: %s).\n", pname)
			return nil
		},
	}
}

func cloudStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}
			pname := resolveProfile(creds)
			p := creds.Profiles[pname]
			if p == nil {
				fmt.Printf("Not logged in (profile: %s). Run: stroppy-cloud cloud login\n", pname)
				return nil
			}
			fmt.Printf("Profile:  %s\n", pname)
			fmt.Printf("Server:   %s\n", p.Server)
			fmt.Printf("Tenant:   %s\n", orNone(p.Tenant))

			tokenStatus := "none"
			if p.AccessToken != "" {
				if isJWTExpired(p.AccessToken) {
					tokenStatus = "expired (re-run login)"
				} else {
					tokenStatus = "valid"
				}
			}
			fmt.Printf("Token:    %s\n", tokenStatus)
			return nil
		},
	}
}

func cloudTenantsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tenants",
		Short: "List tenants the account belongs to (TenantService.ListMyTenants)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()

			ctx, cancel := callCtx()
			defer cancel()
			list, err := uipb.NewTenantServiceClient(c.conn).ListMyTenants(ctx, &emptypb.Empty{})
			if err != nil {
				return fmt.Errorf("list tenants: %w", err)
			}
			tenants := list.GetTenants()
			if len(tenants) == 0 {
				fmt.Println("No tenants.")
				return nil
			}
			for _, t := range tenants {
				marker := " "
				if t.GetEntity().GetId().GetValue() == c.tenant {
					marker = "*"
				}
				fmt.Printf("  %s %s\n", marker, tenantLabel(t))
			}
			return nil
		},
	}
}

func cloudUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <tenant-id>",
		Short: "Switch the active profile's tenant (client-side; no SelectTenant RPC exists)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tenant := args[0]

			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()

			// Validate the tenant is one the account belongs to.
			ctx, cancel := callCtx()
			defer cancel()
			list, err := uipb.NewTenantServiceClient(c.conn).ListMyTenants(ctx, &emptypb.Empty{})
			if err != nil {
				return fmt.Errorf("list tenants: %w", err)
			}
			found := false
			for _, t := range list.GetTenants() {
				if t.GetEntity().GetId().GetValue() == tenant {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("tenant %q not found among your tenants", tenant)
			}

			p := c.creds.Profiles[c.profileName]
			if p == nil {
				return fmt.Errorf("no current profile; run: stroppy-cloud cloud login")
			}
			p.Tenant = tenant
			if err := c.creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
			fmt.Printf("Active tenant: %s\n", tenant)
			return nil
		},
	}
}

func cloudProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List saved credentials profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := loadCredentials()
			if err != nil {
				return err
			}
			if len(creds.Profiles) == 0 {
				fmt.Println("No profiles. Run: stroppy-cloud cloud login")
				return nil
			}
			for name, p := range creds.Profiles {
				marker := " "
				if name == creds.Current {
					marker = "*"
				}
				fmt.Printf("  %s %-15s  %s  %s\n", marker, name, p.Server, orNone(p.Tenant))
			}
			return nil
		},
	}
}

func cloudUseProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use-profile <name>",
		Short: "Switch the default credentials profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			creds, err := loadCredentials()
			if err != nil {
				return err
			}
			if _, ok := creds.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			creds.Current = name
			if err := creds.Save(); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
			fmt.Printf("Default profile: %s\n", name)
			return nil
		},
	}
}

// tenantLabel renders a tenant for listings. The proto Tenant has no display
// name, so identity is its ULID; owner is shown when present.
func tenantLabel(t *models.Tenant) string {
	id := t.GetEntity().GetId().GetValue()
	if owner := t.GetOwnerAccountId().GetValue(); owner != "" {
		return fmt.Sprintf("%s (owner: %s)", id, owner)
	}
	return id
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
