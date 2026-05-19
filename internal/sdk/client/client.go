package client

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/admin/adminconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/agent/agentconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog/catalogconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam/iamconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/ops/opsconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy/stroppyconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system/systemconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing/testingconnect"
)

// Client aggregates typed Connect stubs for every public service.
type Client struct {
	server string

	httpClient *http.Client
	headers    http.Header

	Auth         iamconnect.AuthServiceClient
	User         iamconnect.UserServiceClient
	Tenant       iamconnect.TenantServiceClient
	TenantMember iamconnect.TenantMemberServiceClient
	ApiToken     iamconnect.ApiTokenServiceClient

	DatabasePreset catalogconnect.DatabasePresetServiceClient
	WorkloadPreset catalogconnect.WorkloadPresetServiceClient
	Package        catalogconnect.PackageServiceClient
	Settings       catalogconnect.SettingsServiceClient
	Stroppy        stroppyconnect.StroppyServiceClient
	Schedule       systemconnect.ScheduleServiceClient

	Template       testingconnect.TestRunTemplateServiceClient
	TestRun        testingconnect.TestRunServiceClient
	TestSuite      testingconnect.TestSuiteServiceClient
	TestSuiteRun   testingconnect.TestSuiteRunServiceClient
	SharedTestRun  testingconnect.SharedTestRunServiceClient
	SharedSuiteRun testingconnect.SharedSuiteRunServiceClient
	Comparison     testingconnect.ComparisonServiceClient
	Baseline       testingconnect.BaselineServiceClient

	Agent       agentconnect.AgentServiceClient
	Webhook     opsconnect.WebhookServiceClient
	Quota       opsconnect.QuotaServiceClient
	BinaryCache agentconnect.BinaryCacheServiceClient

	// Admin clients (platform-admin only).
	Admin            adminconnect.AdminServiceClient
	BinaryCacheAdmin adminconnect.BinaryCacheAdminServiceClient
}

// Option configures Client.
type Option func(c *Client)

// WithBearer sets the access-token header on every request.
func WithBearer(token string) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set("Authorization", "Bearer "+token)
	}
}

// WithTenantHeader sets the X-Tenant-Id header on every request so that the
// Tenant middleware can resolve the active tenant without a body field.
func WithTenantHeader(tenantID string) Option {
	return func(c *Client) {
		if c.headers == nil {
			c.headers = http.Header{}
		}
		c.headers.Set("X-Tenant-Id", tenantID)
	}
}

// New builds a Client targeting serverURL.
func New(serverURL string, opts ...Option) *Client {
	c := &Client{server: serverURL, httpClient: http.DefaultClient, headers: http.Header{}}
	for _, opt := range opts {
		opt(c)
	}
	connOpts := []connect.ClientOption{connect.WithInterceptors(headerInterceptor(c.headers))}
	c.Auth = iamconnect.NewAuthServiceClient(c.httpClient, serverURL, connOpts...)
	c.User = iamconnect.NewUserServiceClient(c.httpClient, serverURL, connOpts...)
	c.Tenant = iamconnect.NewTenantServiceClient(c.httpClient, serverURL, connOpts...)
	c.TenantMember = iamconnect.NewTenantMemberServiceClient(c.httpClient, serverURL, connOpts...)
	c.ApiToken = iamconnect.NewApiTokenServiceClient(c.httpClient, serverURL, connOpts...)
	c.DatabasePreset = catalogconnect.NewDatabasePresetServiceClient(c.httpClient, serverURL, connOpts...)
	c.WorkloadPreset = catalogconnect.NewWorkloadPresetServiceClient(c.httpClient, serverURL, connOpts...)
	c.Package = catalogconnect.NewPackageServiceClient(c.httpClient, serverURL, connOpts...)
	c.Settings = catalogconnect.NewSettingsServiceClient(c.httpClient, serverURL, connOpts...)
	c.Stroppy = stroppyconnect.NewStroppyServiceClient(c.httpClient, serverURL, connOpts...)
	c.Schedule = systemconnect.NewScheduleServiceClient(c.httpClient, serverURL, connOpts...)
	c.Webhook = opsconnect.NewWebhookServiceClient(c.httpClient, serverURL, connOpts...)
	c.Quota = opsconnect.NewQuotaServiceClient(c.httpClient, serverURL, connOpts...)
	c.BinaryCache = agentconnect.NewBinaryCacheServiceClient(c.httpClient, serverURL, connOpts...)
	c.Agent = agentconnect.NewAgentServiceClient(c.httpClient, serverURL, connOpts...)
	c.Admin = adminconnect.NewAdminServiceClient(c.httpClient, serverURL, connOpts...)
	c.BinaryCacheAdmin = adminconnect.NewBinaryCacheAdminServiceClient(c.httpClient, serverURL, connOpts...)
	c.initTestingClients(connOpts)
	return c
}

func headerInterceptor(h http.Header) connect.Interceptor {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			for k, vs := range h {
				for _, v := range vs {
					req.Header().Add(k, v)
				}
			}
			return next(ctx, req)
		}
	})
}
