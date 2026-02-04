package crossplane

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/stroppy-io/hatchet-workflow/internal/cloud/crossplane/k8s"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type K8SActor interface {
	CreateResource(
		ctx context.Context,
		request *crossplane.Resource,
	) error
	UpdateResourceFromRemote(
		ctx context.Context,
		resource *crossplane.Resource,
	) (*crossplane.Resource, error)
	DeleteResource(
		ctx context.Context,
		ref *crossplane.ExtRef,
	) error
}

type Service struct {
	k8sActor          K8SActor
	reconcileInterval time.Duration
}

func NewService(
	k8sActor K8SActor,
	reconcileInterval time.Duration,
) *Service {
	return &Service{
		k8sActor:          k8sActor,
		reconcileInterval: reconcileInterval,
	}
}

func (c *Service) CreateDeployment(
	ctx context.Context,
	deployment *crossplane.Deployment,
) (*crossplane.Deployment, error) {
	for _, resource := range deployment.GetCloudDetails().GetResources() {
		resource.Status = crossplane.Resource_STATUS_CREATING
		resource.CreatedAt = timestamppb.Now()
		resource.UpdatedAt = timestamppb.Now()
		err := c.k8sActor.CreateResource(ctx, resource)
		if err != nil {
			return nil, err
		}
	}
	return deployment, nil
}

func (c *Service) ProcessDeploymentStatus(
	ctx context.Context,
	deployment *crossplane.Deployment,
) (*crossplane.Deployment, error) {
	for _, oldResource := range deployment.GetCloudDetails().GetResources() {
		newResource, err := c.k8sActor.UpdateResourceFromRemote(ctx, oldResource)
		if err != nil {
			if errors.Is(err, k8s.ErrResourceNotFound) {
				if oldResource.GetStatus() == crossplane.Resource_STATUS_DESTROYING {
					oldResource.Status = crossplane.Resource_STATUS_DESTROYED
					oldResource.UpdatedAt = timestamppb.Now()
				}
				continue
			}
			return nil, err
		}
		oldResource = newResource
		if IsResourceReady(newResource) {
			oldResource.Status = crossplane.Resource_STATUS_READY
			oldResource.UpdatedAt = timestamppb.Now()
		}
		if slices.Contains(
			[]crossplane.Resource_Status{
				crossplane.Resource_STATUS_CREATING,
				crossplane.Resource_STATUS_DESTROYING,
			},
			oldResource.GetStatus(),
		) &&
			oldResource.GetCreatedAt().AsTime().Add(c.reconcileInterval).Before(time.Now()) {
			oldResource.Status = crossplane.Resource_STATUS_DEGRADED
			oldResource.UpdatedAt = newResource.GetUpdatedAt()
			// delete resource if creating and degrading
			if oldResource.GetStatus() == crossplane.Resource_STATUS_CREATING {
				err := c.k8sActor.DeleteResource(ctx, oldResource.GetRef())
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return deployment, nil
}

func (c *Service) DestroyDeployment(
	ctx context.Context,
	deployment *crossplane.Deployment,
) error {
	for _, node := range deployment.GetCloudDetails().GetResources() {
		node.Status = crossplane.Resource_STATUS_DESTROYING
		err := c.k8sActor.DeleteResource(ctx, node.GetRef())
		if err != nil {
			return err
		}
	}
	return nil
}
