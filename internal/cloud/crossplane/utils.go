package crossplane

import "github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"

func IsResourceReady(resource *crossplane.Resource) bool {
	return resource.GetReady() &&
		resource.GetSynced() &&
		resource.GetExternalId() != ""
}
