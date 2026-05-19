//go:build e2e

package e2e_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/core/ids"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	"github.com/stroppy-io/stroppy-cloud/internal/sdk/client"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/fixture"
	"github.com/stroppy-io/stroppy-cloud/internal/testutil/testserver"
)

// e2eEnv bundles the bits every test needs: full fixture + in-process server
// + an admin user (root) already promoted to PLATFORM_ROLE_ADMIN + an admin
// access token. Per the task spec each test must build its own — call newEnv
// from inside each TestXxx to get an isolated stack.
type e2eEnv struct {
	F           *fixture.AllFixture
	Server      *httptest.Server
	AdminID     *iampb.UserId
	AdminEmail  string
	AdminNick   string
	AdminPass   string
	AdminToken  string
	AnonClient  *client.Client
	AdminClient *client.Client
}

// newEnv stands up a full-stack server backed by a fresh fixture. Bootstraps
// a single platform admin "admin@e2e.test" / nickname "admin" / password
// "Str0ng!Pass" and logs in to obtain an access token. Tests that need
// tenant-scoped clients build them via env.WithTenant(t, tenantID).
func newEnv(t *testing.T, releasesJSON, commitsJSON string) *e2eEnv {
	t.Helper()
	ctx := context.Background()

	f := fixture.NewAll(t, releasesJSON, commitsJSON)
	srv := httptest.NewServer(testserver.ForAll(testserver.AllDeps{
		IAM:             f.IAM,
		Catalog:         f.Catalog,
		Stroppy:         f.Stroppy,
		System:          f.System,
		Templates:       f.Templates,
		Runs:            f.Runs,
		Suites:          f.Suites,
		SuiteRuns:       f.SuiteRuns,
		SharedRuns:      f.SharedRuns,
		SharedSuiteRuns: f.SharedSuiteRuns,
		Comparison:      f.Comparison,
		Baselines:       f.Baselines,
		Webhooks:        f.Webhooks,
		Quota:           f.Quota,
		BinaryCache:     f.BinaryCache,
		Admin:           f.Admin,
		BinaryAdmin:     f.BinaryAdmin,
		Agent:           f.Agent,
	}))
	t.Cleanup(srv.Close)

	// pgcontainer.Bootstrap shares a single schema across tests in the package,
	// so user emails/nicknames must be unique per test to avoid CODE_ALREADY_EXISTS.
	suffix := strings.ToLower(ids.New())
	email := "admin-" + suffix + "@e2e.test"
	nick := "admin" + suffix
	const pass = "Str0ng!Pass"
	u, err := f.IAM.CreateUser(ctx, &iampb.User{Email: email, Nickname: nick}, pass)
	require.NoError(t, err)
	_, err = f.IAM.PromoteToAdmin(ctx, u.GetId())
	require.NoError(t, err)

	anon := client.New(srv.URL)
	resp, err := anon.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: email, Password: pass}))
	require.NoError(t, err)
	access := resp.Msg.GetTokens().GetAccessToken()
	require.NotEmpty(t, access)

	return &e2eEnv{
		F:           f,
		Server:      srv,
		AdminID:     u.GetId(),
		AdminEmail:  email,
		AdminNick:   nick,
		AdminPass:   pass,
		AdminToken:  access,
		AnonClient:  anon,
		AdminClient: client.New(srv.URL, client.WithBearer(access)),
	}
}

// WithTenant returns a Client bearing the admin token + X-Tenant-Id for the
// given tenant.
func (e *e2eEnv) WithTenant(tenantID string) *client.Client {
	return client.New(e.Server.URL, client.WithBearer(e.AdminToken), client.WithTenantHeader(tenantID))
}

// WithTokenAndTenant returns a Client bearing a specific token + X-Tenant-Id.
func (e *e2eEnv) WithTokenAndTenant(token, tenantID string) *client.Client {
	return client.New(e.Server.URL, client.WithBearer(token), client.WithTenantHeader(tenantID))
}

// LoginAs creates an unprivileged user with a unique per-call suffix appended
// to the supplied email/nick so the shared schema does not collide. Returns
// the user's id, access token, and the actual email used (with suffix).
func (e *e2eEnv) LoginAs(t *testing.T, email, nick, pass string) (*iampb.UserId, string) {
	t.Helper()
	id, _, tok := e.LoginAsFull(t, email, nick, pass)
	return id, tok
}

// LoginAsFull mirrors LoginAs but also returns the unique email assigned.
func (e *e2eEnv) LoginAsFull(t *testing.T, email, nick, pass string) (*iampb.UserId, string, string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ToLower(ids.New())
	uniqEmail := suffix + "+" + email
	uniqNick := nick + "-" + suffix
	u, err := e.F.IAM.CreateUser(ctx, &iampb.User{Email: uniqEmail, Nickname: uniqNick}, pass)
	require.NoError(t, err)
	resp, err := e.AnonClient.Auth.Login(ctx, connect.NewRequest(&iampb.LoginRequest{Email: uniqEmail, Password: pass}))
	require.NoError(t, err)
	return u.GetId(), uniqEmail, resp.Msg.GetTokens().GetAccessToken()
}

// SeedTenant creates a fresh tenant owned by the admin (suffix appended to
// name to avoid cross-test collisions) and returns its id.
func (e *e2eEnv) SeedTenant(t *testing.T, name string) *iampb.TenantId {
	t.Helper()
	uniq := name + "-" + strings.ToLower(ids.New())
	id, err := e.F.SeedTenant(context.Background(), uniq, e.AdminID)
	require.NoError(t, err)
	return id
}
