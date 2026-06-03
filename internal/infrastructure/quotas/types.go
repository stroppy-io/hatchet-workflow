package quotas

import (
	"context"
	"time"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	workflowpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/workflow"
)

const (
	ResourceTypeYandexCloud = "resource-manager.cloud"
	ResourceTypeDockerHost  = "docker.host"
)

type Scope struct {
	TenantID     string
	Provider     deploymentpb.Provider
	ResourceType string
	ResourceID   string
	Service      string
}

type Snapshot struct {
	Scope
	QuotaName         string
	Units             string
	ProviderUsed      float64
	Limit             float64
	ProviderAvailable float64
	ObservedAt        time.Time
	StaleAfter        time.Time
	Raw               []byte
}

type Reservation struct {
	ID           string
	TenantID     string
	RunID        string
	NodeID       string
	Provider     deploymentpb.Provider
	ResourceType string
	ResourceID   string
	Service      string
	QuotaName    string
	Units        string
	Amount       uint64
	Status       deploymentpb.Quota_ReservationStatus
	WorkflowID   string
	ExpiresAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ReserveInput struct {
	TenantID       string
	RunID          string
	WorkflowID     string
	Scope          Scope
	QuotaRequests  []*workflowpb.QuotaRequestRef
	ReservationTTL time.Duration
	Now            time.Time
}

type SourceRequest struct {
	TenantID string
	Scope    Scope
	Settings *deploymentpb.ProviderSettings
	Services []string
}

type ProviderSource interface {
	ListQuotas(ctx context.Context, req SourceRequest) ([]Snapshot, error)
}

type Views struct {
	Quotas       []*api.QuotaView
	Reservations []*api.QuotaReservationView
}
