//go:build integration

package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/gopherex/pgtx"
	"github.com/gopherex/pgtx/pkg/tx"
	"github.com/gopherex/xlog"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stroppy-io/stroppy-cloud/internal/api/caller"
	domainauth "github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	adminpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/admin"
	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
	"github.com/stroppy-io/stroppy-cloud/internal/services/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/services/tenancy"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil"
)

var (
	testContainer *testutil.PostgresContainer
	valkeyAddr    string // host:port of the shared valkey container
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, err := testutil.NewPostgresContainer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		os.Exit(1)
	}
	defer pg.Close(ctx)
	if err := pg.CreateTemplateDB(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "create template db: %v\n", err)
		os.Exit(1)
	}
	testContainer = pg

	vk, addr, err := startValkey(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start valkey: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = vk.Terminate(ctx) }()
	valkeyAddr = addr

	os.Exit(m.Run())
}

// startValkey runs a single-node valkey container (valkey-go speaks the redis
// protocol). It is generic rather than the redis testcontainers module to avoid
// pulling a new dependency, per the harness conventions.
func startValkey(ctx context.Context) (testcontainers.Container, string, error) {
	const port = "6379/tcp"
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "valkey/valkey:8-alpine",
			ExposedPorts: []string{port},
			WaitingFor:   wait.ForListeningPort(nat.Port(port)).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("start valkey container: %w", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, "", fmt.Errorf("valkey host: %w", err)
	}
	mapped, err := c.MappedPort(ctx, nat.Port(port))
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, "", fmt.Errorf("valkey port: %w", err)
	}
	return c, fmt.Sprintf("%s:%s", host, mapped.Port()), nil
}

// testConfig is a static auth.Config: short access TTL, longer refresh TTL.
type testConfig struct{}

func (testConfig) AccessTokenTTL() time.Duration  { return 15 * time.Minute }
func (testConfig) RefreshTokenTTL() time.Duration { return time.Hour }
func (testConfig) JWTSecret() []byte              { return []byte("integration-test-secret-key-0123456789") }

// fixture bundles the AuthService under test plus the tenancy admin services used
// to seed the account it authenticates against.
type fixture struct {
	svc     *auth.AuthService
	acctSvc *tenancy.AccountAdminService
	cfg     auth.Config
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testContainer.NewTestDB(t)
	trm, err := pgtx.NewTxManager(db.Pool, tx.ReadCommitted())
	require.NoError(t, err)
	executor := pgtx.NewTxDB(db.Pool)
	log := xlog.Default()

	// A dedicated valkey client per test; sessions are keyed by token hash so
	// the shared container does not leak state across tests in practice.
	vk, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  []string{valkeyAddr},
		DisableCache: true, // no RESP3 client-side caching needed for tests
	})
	require.NoError(t, err)
	t.Cleanup(vk.Close)
	// Flush so each test starts from a clean session store.
	require.NoError(t, vk.Do(context.Background(), vk.B().Flushall().Build()).Error())

	cfg := testConfig{}
	return &fixture{
		svc:     auth.New(log, executor, vk, cfg),
		acctSvc: tenancy.NewAccountAdminService(log, executor, trm),
		cfg:     cfg,
	}
}

const (
	testEmail    = "alice@example.com"
	testNickname = "alice"
	testPassword = "s3cret-pass"
)

// seedAccount creates an account with the canonical test credentials and returns
// its id. CreateAccount hashes the password into the write-only column.
func (f *fixture) seedAccount(t *testing.T, isAdmin bool) string {
	t.Helper()
	acc, err := f.acctSvc.CreateAccount(context.Background(), &adminpb.CreateAccountRequest{
		Account:  &models.Account{Email: testEmail, Nickname: testNickname, IsAdmin: isAdmin},
		Password: testPassword,
	})
	require.NoError(t, err)
	id := acc.GetEntity().GetId().GetValue()
	require.NotEmpty(t, id)
	return id
}

// TestLoginSuccess covers the happy path: correct credentials yield a token pair.
func TestLoginSuccess(t *testing.T) {
	f := newFixture(t)
	f.seedAccount(t, false)
	ctx := context.Background()

	resp, err := f.svc.Login(ctx, &uipb.LoginRequest{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	pair := resp.GetTokens()
	require.NotNil(t, pair)
	require.NotEmpty(t, pair.GetAccessToken())
	require.NotEmpty(t, pair.GetRefreshToken())
	require.Equal(t, f.cfg.AccessTokenTTL(), pair.GetAccessTokenExpiresIn().AsDuration())
	require.Equal(t, f.cfg.RefreshTokenTTL(), pair.GetRefreshTokenExpiresIn().AsDuration())
}

// TestLoginByNickname: the email field also accepts the nickname (getAccountByLogin
// falls back to a nickname lookup).
func TestLoginByNickname(t *testing.T) {
	f := newFixture(t)
	f.seedAccount(t, false)

	resp, err := f.svc.Login(context.Background(), &uipb.LoginRequest{Email: testNickname, Password: testPassword})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetTokens().GetAccessToken())
}

// TestLoginWrongPassword: wrong password is Unauthenticated.
func TestLoginWrongPassword(t *testing.T) {
	f := newFixture(t)
	f.seedAccount(t, false)

	_, err := f.svc.Login(context.Background(), &uipb.LoginRequest{Email: testEmail, Password: "wrong-password"})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestLoginUnknownAccount: an unknown login is Unauthenticated (and runs the
// constant-time bcrypt path so it does not error out on the nil hash).
func TestLoginUnknownAccount(t *testing.T) {
	f := newFixture(t)
	// no account seeded
	_, err := f.svc.Login(context.Background(), &uipb.LoginRequest{Email: "ghost@example.com", Password: testPassword})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestAccessTokenAuthenticates: the issued access JWT resolves to the account
// principal through the Authenticator (the same code path the gRPC middleware uses).
func TestAccessTokenAuthenticates(t *testing.T) {
	f := newFixture(t)
	accountID := f.seedAccount(t, true)
	ctx := context.Background()

	resp, err := f.svc.Login(ctx, &uipb.LoginRequest{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	access := resp.GetTokens().GetAccessToken()

	authCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+access))
	c, err := f.svc.Authenticate(authCtx)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, caller.PrincipalAccount, c.Kind)
	require.Equal(t, accountID, c.AccountID.GetValue())
	require.True(t, c.IsAdmin) // is_admin propagated into the JWT claim

	// Me resolves the account behind the principal in context.
	meCtx := caller.NewContext(ctx, c)
	me, err := f.svc.Me(meCtx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, accountID, me.GetEntity().GetId().GetValue())
	require.Equal(t, testEmail, me.GetEmail())
}

// TestAnonymousAuthenticate: no Authorization header -> anonymous (nil, nil), not
// an error. Guards/services decide who may proceed.
func TestAnonymousAuthenticate(t *testing.T) {
	f := newFixture(t)
	c, err := f.svc.Authenticate(context.Background())
	require.NoError(t, err)
	require.Nil(t, c)
}

// TestInvalidJWTRejected: a bearer that is neither a valid JWT nor a known API
// token hash is rejected as invalid credentials.
func TestInvalidJWTRejected(t *testing.T) {
	f := newFixture(t)
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs("authorization", "Bearer not-a-real-jwt-or-token"))
	_, err := f.svc.Authenticate(ctx)
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRefreshTokenRoundTrip: a valid refresh token rotates into a new pair, and
// the old refresh token is invalidated (rotation).
func TestRefreshTokenRoundTrip(t *testing.T) {
	f := newFixture(t)
	f.seedAccount(t, false)
	ctx := context.Background()

	login, err := f.svc.Login(ctx, &uipb.LoginRequest{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	firstRefresh := login.GetTokens().GetRefreshToken()

	// Rotate.
	refreshed, err := f.svc.RefreshTokens(ctx, &uipb.RefreshTokenRequest{RefreshToken: firstRefresh})
	require.NoError(t, err)
	require.NotEmpty(t, refreshed.GetTokens().GetAccessToken())
	newRefresh := refreshed.GetTokens().GetRefreshToken()
	require.NotEmpty(t, newRefresh)
	require.NotEqual(t, firstRefresh, newRefresh, "refresh token must rotate")

	// The consumed refresh token is now rejected (one-time use).
	_, err = f.svc.RefreshTokens(ctx, &uipb.RefreshTokenRequest{RefreshToken: firstRefresh})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// The new refresh token still works.
	_, err = f.svc.RefreshTokens(ctx, &uipb.RefreshTokenRequest{RefreshToken: newRefresh})
	require.NoError(t, err)
}

// TestRefreshMissingToken: an empty refresh token is Unauthenticated.
func TestRefreshMissingToken(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.RefreshTokens(context.Background(), &uipb.RefreshTokenRequest{RefreshToken: ""})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRefreshUnknownToken: a refresh token with no matching valkey session is
// Unauthenticated.
func TestRefreshUnknownToken(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.RefreshTokens(context.Background(),
		&uipb.RefreshTokenRequest{RefreshToken: "never-issued-this-token"})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestLogoutInvalidatesSession: after Logout, the refresh token can no longer be
// used to mint a new pair.
func TestLogoutInvalidatesSession(t *testing.T) {
	f := newFixture(t)
	f.seedAccount(t, false)
	ctx := context.Background()

	login, err := f.svc.Login(ctx, &uipb.LoginRequest{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	refresh := login.GetTokens().GetRefreshToken()

	_, err = f.svc.Logout(ctx, &uipb.LogoutRequest{RefreshToken: refresh})
	require.NoError(t, err)

	_, err = f.svc.RefreshTokens(ctx, &uipb.RefreshTokenRequest{RefreshToken: refresh})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestLogoutEmptyTokenIsNoop: Logout with an empty token is a no-op success
// (nothing to revoke).
func TestLogoutEmptyTokenIsNoop(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Logout(context.Background(), &uipb.LogoutRequest{RefreshToken: ""})
	require.NoError(t, err)
}

// TestMeWithoutPrincipal: Me with no account principal in context is
// Unauthenticated.
func TestMeWithoutPrincipal(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Me(context.Background(), &emptypb.Empty{})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRefreshSessionPersistedInValkey asserts the refresh session is actually
// written to valkey under sha256(token), independent of the service helpers.
func TestRefreshSessionPersistedInValkey(t *testing.T) {
	f := newFixture(t)
	f.seedAccount(t, false)
	ctx := context.Background()

	login, err := f.svc.Login(ctx, &uipb.LoginRequest{Email: testEmail, Password: testPassword})
	require.NoError(t, err)
	refresh := login.GetTokens().GetRefreshToken()

	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{valkeyAddr}, DisableCache: true})
	require.NoError(t, err)
	defer vk.Close()

	key := "auth:refresh:" + domainauth.HashToken(refresh)
	raw, err := vk.Do(ctx, vk.B().Get().Key(key).Build()).ToString()
	require.NoError(t, err, "refresh session must exist in valkey under sha256(token)")
	require.Contains(t, raw, `"aid"`)

	// And a TTL is set (refresh sessions expire).
	ttl, err := vk.Do(ctx, vk.B().Ttl().Key(key).Build()).ToInt64()
	require.NoError(t, err)
	require.Greater(t, ttl, int64(0))
}
