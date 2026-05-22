package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

// ── global flags (bound on the `cloud` parent persistent flag set) ──────────

var (
	cloudServer  string
	cloudProfile string
	cloudTenant  string
)

const defaultServer = "localhost:8080"

// newCloudCmd builds the operator CLI subtree: a `cloud` parent command that
// talks to the control plane over gRPC. Every leaf command dials the server
// with a bearer-token interceptor sourced from the stored credentials profile.
func newCloudCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Operate a remote stroppy-cloud control plane (gRPC)",
	}

	cmd.PersistentFlags().StringVar(&cloudServer, "server", "", "server address host:port (env: STROPPY_SERVER, default "+defaultServer+")")
	cmd.PersistentFlags().StringVar(&cloudProfile, "profile", "", "credentials profile (env: STROPPY_PROFILE)")
	cmd.PersistentFlags().StringVar(&cloudTenant, "tenant", "", "tenant id override (defaults to the profile's stored tenant)")

	cmd.AddCommand(
		cloudLoginCmd(),
		cloudLogoutCmd(),
		cloudStatusCmd(),
		cloudTenantsCmd(),
		cloudUseCmd(),
		cloudProfilesCmd(),
		cloudUseProfileCmd(),
		cloudRunCmd(),
		cloudWaitCmd(),
		cloudCompareCmd(),
		cloudBenchCmd(),
		cloudPackagesCmd(),
		cloudPresetsCmd(),
		cloudProbeCmd(),
	)
	return cmd
}

// ── credentials store (~/.config/stroppy-cloud/credentials.json) ────────────

// Profile is one named set of server + tenant + tokens. Tenant is a tenant
// ULID (the proto Tenant has no display name; identity is Entity.Id).
type Profile struct {
	Server       string `json:"server"`
	Tenant       string `json:"tenant,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type Credentials struct {
	Profiles map[string]*Profile `json:"profiles"`
	Current  string              `json:"current"`
}

func credentialsPath() string {
	if dir := os.Getenv("STROPPY_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "credentials.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "stroppy-cloud", "credentials.json")
}

func loadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Credentials{Profiles: map[string]*Profile{}, Current: "default"}, nil
		}
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	if c.Current == "" {
		c.Current = "default"
	}
	return &c, nil
}

func (c *Credentials) Save() error {
	path := credentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Credentials) CurrentProfile() *Profile {
	if p, ok := c.Profiles[c.Current]; ok {
		return p
	}
	return nil
}

// resolveProfile picks the active profile name from --profile, env, or the
// stored `current` pointer.
func resolveProfile(creds *Credentials) string {
	if cloudProfile != "" {
		return cloudProfile
	}
	if p := os.Getenv("STROPPY_PROFILE"); p != "" {
		return p
	}
	return creds.Current
}

// ── gRPC client wrapper ─────────────────────────────────────────────────────

// cloudClient bundles a dialed gRPC connection with the resolved server,
// tenant and the backing credentials store (so token refresh can persist).
type cloudClient struct {
	conn        *grpc.ClientConn
	server      string
	tenant      string
	token       string
	creds       *Credentials
	profileName string
}

// resolveServer applies the --server flag, the STROPPY_SERVER env, the stored
// profile, then the default. A leading scheme is stripped because gRPC dials a
// bare host:port.
func resolveServer(prof *Profile) string {
	s := cloudServer
	if s == "" {
		s = os.Getenv("STROPPY_SERVER")
	}
	if s == "" && prof != nil {
		s = prof.Server
	}
	if s == "" {
		s = defaultServer
	}
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	return strings.TrimRight(s, "/")
}

// resolveTenant applies --tenant, else the profile's stored tenant.
func resolveTenant(prof *Profile) string {
	if cloudTenant != "" {
		return cloudTenant
	}
	if prof != nil {
		return prof.Tenant
	}
	return ""
}

// bearerInterceptor injects "authorization: Bearer <token>" metadata on every
// unary call. An empty token leaves the request anonymous (public RPCs work).
func bearerInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// dialClient builds an authenticated gRPC client from the active profile.
func dialClient() (*cloudClient, error) {
	creds, err := loadCredentials()
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	pname := resolveProfile(creds)
	prof := creds.Profiles[pname]

	server := resolveServer(prof)
	token := ""
	if prof != nil {
		token = prof.AccessToken
	}
	tenant := resolveTenant(prof)

	conn, err := grpc.NewClient(
		server,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(bearerInterceptor(token)),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", server, err)
	}

	return &cloudClient{
		conn:        conn,
		server:      server,
		tenant:      tenant,
		token:       token,
		creds:       creds,
		profileName: pname,
	}, nil
}

func (c *cloudClient) close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// tenantID returns the resolved tenant wrapped as a *models.TenantId, or an
// error when no tenant is set (most tenant-scoped RPCs require it).
func (c *cloudClient) tenantID() (*models.TenantId, error) {
	if c.tenant == "" {
		return nil, fmt.Errorf("no tenant selected; pass --tenant <id> or run: stroppy-cloud cloud use <tenant-id>")
	}
	return &models.TenantId{Value: c.tenant}, nil
}

// dialAnon builds an unauthenticated client (no stored token needed). Used by
// login before any credentials exist.
func dialAnon(server string) (*grpc.ClientConn, error) {
	server = strings.TrimPrefix(server, "http://")
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimRight(server, "/")
	return grpc.NewClient(
		server,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// dialAnonWithToken dials server with a bearer interceptor for the given token.
// Used before credentials are saved (login) and for one-shot calls (logout).
func dialAnonWithToken(server, token string) (*grpc.ClientConn, error) {
	server = strings.TrimPrefix(server, "http://")
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimRight(server, "/")
	return grpc.NewClient(
		server,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(bearerInterceptor(token)),
	)
}

// callCtx returns a context with a sane default timeout for unary calls.
func callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// ── token helpers ───────────────────────────────────────────────────────────

// jwtExpiry returns the JWT `exp` (unix seconds) if the token is a parseable
// JWT, else (0, false).
func jwtExpiry(token string) (int64, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, false
	}
	return claims.Exp, claims.Exp != 0
}

func isJWTExpired(token string) bool {
	exp, ok := jwtExpiry(token)
	if !ok {
		return false
	}
	return time.Now().Unix() > exp-30
}
