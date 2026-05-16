package client

import (
	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/testing/testingconnect"
)

// initTestingClients wires all testing-domain Connect stubs onto c.
func (c *Client) initTestingClients(connOpts []connect.ClientOption) {
	c.Template = testingconnect.NewTestRunTemplateServiceClient(c.httpClient, c.server, connOpts...)
	c.TestRun = testingconnect.NewTestRunServiceClient(c.httpClient, c.server, connOpts...)
	c.TestSuite = testingconnect.NewTestSuiteServiceClient(c.httpClient, c.server, connOpts...)
	c.TestSuiteRun = testingconnect.NewTestSuiteRunServiceClient(c.httpClient, c.server, connOpts...)
	c.SharedTestRun = testingconnect.NewSharedTestRunServiceClient(c.httpClient, c.server, connOpts...)
	c.SharedSuiteRun = testingconnect.NewSharedSuiteRunServiceClient(c.httpClient, c.server, connOpts...)
	c.Comparison = testingconnect.NewComparisonServiceClient(c.httpClient, c.server, connOpts...)
}
