package deployment

import "github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"

type hasResource interface {
	GetResource() *crossplane.Resource
}

func GetResourceMany[T hasResource](getters ...T) []*crossplane.Resource {
	ret := make([]*crossplane.Resource, 0)
	for _, getter := range getters {
		ret = append(ret, getter.GetResource())
	}
	return ret
}

func GetDeploymentResources(deployment *crossplane.Deployment) []*crossplane.Resource {
	return append(append(
		GetResourceMany(deployment.GetVms()...),
		GetResourceMany(deployment.GetNetwork().GetSubnets()...)...,
	), deployment.GetNetwork().GetResource())
}

func GetDeploymentUsingQuotas(deployment *crossplane.Deployment) []*crossplane.Quota {
	ret := make([]*crossplane.Quota, 0)
	for _, resource := range GetDeploymentResources(deployment) {
		ret = append(ret, resource.GetUsingQuotas()...)
	}
	return ret
}
