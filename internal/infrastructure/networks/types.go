package networks

import (
	"context"
	"time"

	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
)

const (
	ResourceTypeYandexNetwork = "vpc.network"
	defaultYandexCIDRPool     = "10.0.0.0/8"
)

type Scope struct {
	TenantID     string
	Provider     deploymentpb.Provider
	ResourceType string
	ResourceID   string
}

type ProviderSubnet struct {
	ID    string
	Name  string
	Zone  string
	CIDRs []string
	Raw   []byte
}

type SourceRequest struct {
	TenantID string
	Scope    Scope
	Settings *deploymentpb.ProviderSettings
}

type ProviderSource interface {
	ListSubnets(ctx context.Context, req SourceRequest) ([]ProviderSubnet, error)
}

type Reservation struct {
	ID           string
	TenantID     string
	RunID        string
	Provider     deploymentpb.Provider
	ResourceType string
	ResourceID   string
	CIDR         string
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
	BaseCIDR       string
	ProviderCIDRs  []string
	ReservationTTL time.Duration
	Now            time.Time
}
