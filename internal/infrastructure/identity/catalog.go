package identity

import (
	"context"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
	iamsvc "github.com/stroppy-io/stroppy-cloud/internal/services/iam"
)

// Catalog implements iamsvc.PermissionCatalog with a static, build-time catalog
// enumerated directly from the cloud.v1.iam Resource and Action proto enums:
// every concrete resource crossed with every concrete action, plus the MANAGE
// wildcard per resource. UNSPECIFIED values are excluded (never grantable).
type Catalog struct {
	entries []*api.CatalogEntry
}

var _ iamsvc.PermissionCatalog = (*Catalog)(nil)

// catalogResources is every grantable resource (UNSPECIFIED excluded), sourced
// from the iam.Resource enum.
var catalogResources = []iam.Resource{
	iam.Resource_RESOURCE_ACCOUNT,
	iam.Resource_RESOURCE_TENANT,
	iam.Resource_RESOURCE_ROLE,
	iam.Resource_RESOURCE_MEMBERSHIP,
	iam.Resource_RESOURCE_SETTINGS,
	iam.Resource_RESOURCE_PRESET,
	iam.Resource_RESOURCE_WIZARD,
	iam.Resource_RESOURCE_TEST_RUN,
	iam.Resource_RESOURCE_SUITE,
	iam.Resource_RESOURCE_SUITE_RUN,
	iam.Resource_RESOURCE_FAVORITE,
	iam.Resource_RESOURCE_AGENT_SHELL,
	iam.Resource_RESOURCE_SHARE,
	iam.Resource_RESOURCE_PACKAGE,
}

// catalogActions is every grantable action (UNSPECIFIED excluded), sourced from
// the iam.Action enum. MANAGE is the wildcard verb implying all of the others.
var catalogActions = []iam.Action{
	iam.Action_ACTION_CREATE,
	iam.Action_ACTION_READ,
	iam.Action_ACTION_UPDATE,
	iam.Action_ACTION_DELETE,
	iam.Action_ACTION_LIST,
	iam.Action_ACTION_MANAGE,
}

// NewCatalog builds the static catalog once.
func NewCatalog() *Catalog {
	entries := make([]*api.CatalogEntry, 0, len(catalogResources)*len(catalogActions))
	for _, res := range catalogResources {
		for _, act := range catalogActions {
			p := &iam.Permission{Resource: res, Action: act}
			entries = append(entries, &api.CatalogEntry{Permission: p, Label: catalogLabel(p)})
		}
	}
	return &Catalog{entries: entries}
}

// Grantable returns the full static catalog. The context is unused — the catalog
// is derived from compile-time enums.
func (c *Catalog) Grantable(_ context.Context) ([]*api.CatalogEntry, error) {
	return c.entries, nil
}

func catalogLabel(p *iam.Permission) string {
	return actionWord(p.GetAction()) + " " + resourceWord(p.GetResource())
}

func actionWord(a iam.Action) string {
	switch a {
	case iam.Action_ACTION_CREATE:
		return "Create"
	case iam.Action_ACTION_READ:
		return "Read"
	case iam.Action_ACTION_UPDATE:
		return "Update"
	case iam.Action_ACTION_DELETE:
		return "Delete"
	case iam.Action_ACTION_LIST:
		return "List"
	case iam.Action_ACTION_MANAGE:
		return "Manage"
	default:
		return "Access"
	}
}

func resourceWord(r iam.Resource) string {
	switch r {
	case iam.Resource_RESOURCE_ACCOUNT:
		return "account"
	case iam.Resource_RESOURCE_TENANT:
		return "tenant"
	case iam.Resource_RESOURCE_ROLE:
		return "role"
	case iam.Resource_RESOURCE_MEMBERSHIP:
		return "membership"
	case iam.Resource_RESOURCE_SETTINGS:
		return "settings"
	case iam.Resource_RESOURCE_PRESET:
		return "preset"
	case iam.Resource_RESOURCE_WIZARD:
		return "wizard"
	case iam.Resource_RESOURCE_TEST_RUN:
		return "test run"
	case iam.Resource_RESOURCE_SUITE:
		return "suite"
	case iam.Resource_RESOURCE_SUITE_RUN:
		return "suite run"
	case iam.Resource_RESOURCE_FAVORITE:
		return "favorite"
	case iam.Resource_RESOURCE_AGENT_SHELL:
		return "agent shell"
	case iam.Resource_RESOURCE_SHARE:
		return "share"
	case iam.Resource_RESOURCE_PACKAGE:
		return "package"
	default:
		return "resource"
	}
}
