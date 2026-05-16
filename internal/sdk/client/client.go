package client

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog/catalogconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam/iamconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/stroppy/stroppyconnect"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/system/systemconnect"
)

// Client aggregates typed Connect stubs for every public service.
type Client struct {
	server string

	httpClient *http.Client
	headers    http.Header

	Auth   iamconnect.AuthServiceClient
	User   iamconnect.UserServiceClient
	Tenant iamconnect.TenantServiceClient

	DatabasePreset catalogconnect.DatabasePresetServiceClient
	WorkloadPreset catalogconnect.WorkloadPresetServiceClient
	Package        catalogconnect.PackageServiceClient
	Settings       catalogconnect.SettingsServiceClient
	Stroppy        stroppyconnect.StroppyServiceClient
	Schedule       systemconnect.ScheduleServiceClient
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
	c.DatabasePreset = catalogconnect.NewDatabasePresetServiceClient(c.httpClient, serverURL, connOpts...)
	c.WorkloadPreset = catalogconnect.NewWorkloadPresetServiceClient(c.httpClient, serverURL, connOpts...)
	c.Package = catalogconnect.NewPackageServiceClient(c.httpClient, serverURL, connOpts...)
	c.Settings = catalogconnect.NewSettingsServiceClient(c.httpClient, serverURL, connOpts...)
	c.Stroppy = stroppyconnect.NewStroppyServiceClient(c.httpClient, serverURL, connOpts...)
	c.Schedule = systemconnect.NewScheduleServiceClient(c.httpClient, serverURL, connOpts...)
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
