package deployment

import (
	"context"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

// DeploymentService abstracts creating, monitoring, and destroying deployments.
// Implemented by crossplane.Service (cloud mode) and docker.Service (docker mode).
type DeploymentService interface {
	CreateDeployment(ctx context.Context, depl *crossplane.Deployment) (*crossplane.Deployment, error)
	ProcessDeploymentStatus(ctx context.Context, depl *crossplane.Deployment) (*crossplane.Deployment, error)
	DestroyDeployment(ctx context.Context, depl *crossplane.Deployment) error
}
