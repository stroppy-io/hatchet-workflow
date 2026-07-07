package quotas

import (
	"context"
	"time"

	api "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
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

// QuotaAmount is one resource-kind demand Manager.Reserve computes from a
// CompiledPlan's machine groups (see reserve.go's amountsForGroups/
// buildQuotaAmounts) — one row per resource kind for the whole run (not per
// node/machine: a machine_groups-derived demand is an aggregate across every
// machine the group asks for, see reserve.go's doc comment on why reservation
// rows carry an empty NodeID). QuotaName/Units are the provider-specific quota
// a live quota_snapshots row is keyed/reported in (see quotaDimensionsFor);
// Amount is already expressed in those Units.
type QuotaAmount struct {
	QuotaName string
	Units     string
	Amount    uint64
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
