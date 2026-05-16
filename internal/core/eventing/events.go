package eventing

// Topic constants for all domain events.
const (
	TopicUserCreated    Topic = "user.created"
	TopicTenantCreated  Topic = "tenant.created"
	TopicMemberAdded    Topic = "tenant.member.added"
	TopicTestRunLaunched Topic = "testrun.launched"
	TopicTestRunDone    Topic = "testrun.done"
	TopicNodeRunDone    Topic = "noderun.done"
	TopicDagRunDone     Topic = "dagrun.done"
	TopicWebhookEmit    Topic = "webhook.emit"
)

// UserCreated is published when a new user account is created.
type UserCreated struct {
	UserID string
}

// TenantCreated is published when a new tenant is provisioned.
type TenantCreated struct {
	TenantID string
}

// MemberAdded is published when a user is added to a tenant.
type MemberAdded struct {
	TenantID string
	UserID   string
	Role     string
}

// TestRunLaunched is published when a test run is started.
type TestRunLaunched struct {
	TestRunID string
	TenantID  string
}

// TestRunDone is published when a test run finishes.
type TestRunDone struct {
	TestRunID string
	TenantID  string
	Success   bool
}

// NodeRunDone is published when an individual node run completes.
type NodeRunDone struct {
	NodeRunID string
	DagRunID  string
	TestRunID string
	Success   bool
}

// DagRunDone is published when a DAG run completes.
type DagRunDone struct {
	DagRunID  string
	TestRunID string
	Success   bool
}

// WebhookEmit is published to trigger outbound webhook delivery.
type WebhookEmit struct {
	WebhookID string
	Payload   any
}
