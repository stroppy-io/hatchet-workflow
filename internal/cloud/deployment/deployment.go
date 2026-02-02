package deployment

import "github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"

func NewVmDeployment(toBuild *crossplane.Deployment) (*crossplane.Deployment, error) {
	switch toBuild.GetDeployment().(type) {
	case *crossplane.Deployment_Vm_:
		return toBuild
	default:
		return nil
	}
}
