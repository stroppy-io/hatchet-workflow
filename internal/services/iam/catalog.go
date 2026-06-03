package iam

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

/*
	Catalog assembles the grantable-permission catalog by reflecting over the
	(cloud.v1.iam.auth) annotations of EVERY service registered in the build, not
	just IamAPI (see iam/options.proto). For each referenced Resource it also
	synthesises a {resource, ACTION_MANAGE} entry, since MANAGE is grantable on a
	role yet never named in an annotation.
*/

type Catalog struct{}

func NewCatalog() *Catalog { return &Catalog{} }

var _ PermissionCatalog = (*Catalog)(nil)

// Grantable returns the catalog. It is derived from static descriptors, so the
// context is unused; it is built fresh on each call.
func (c *Catalog) Grantable(_ context.Context) ([]*api.CatalogEntry, error) {
	seen := make(map[[2]int32]bool)
	resources := make(map[iam.Resource]bool)
	var entries []*api.CatalogEntry

	add := func(p *iam.Permission) {
		key := [2]int32{int32(p.GetResource()), int32(p.GetAction())}
		if seen[key] {
			return
		}
		seen[key] = true
		entries = append(entries, &api.CatalogEntry{Permission: p, Label: catalogLabel(p)})
	}

	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			methods := services.Get(i).Methods()
			for j := 0; j < methods.Len(); j++ {
				opts := methods.Get(j).Options()
				if opts == nil {
					continue
				}
				ma, ok := proto.GetExtension(opts, iam.E_Auth).(*iam.MethodAuth)
				if !ok || ma == nil {
					continue
				}
				for _, p := range ma.GetAllOf() {
					resources[p.GetResource()] = true
					add(p)
				}
			}
		}
		return true
	})

	// Synthesise the MANAGE wildcard for every resource that appears.
	for res := range resources {
		add(&iam.Permission{Resource: res, Action: iam.Action_ACTION_MANAGE})
	}
	return entries, nil
}

func catalogManagePermissions(ctx context.Context, catalog PermissionCatalog) ([]*iam.Permission, error) {
	entries, err := catalog.Grantable(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[iam.Resource]bool, len(entries))
	perms := make([]*iam.Permission, 0, len(entries))
	for _, entry := range entries {
		p := entry.GetPermission()
		if p.GetAction() != iam.Action_ACTION_MANAGE {
			continue
		}
		if seen[p.GetResource()] {
			continue
		}
		seen[p.GetResource()] = true
		perms = append(perms, &iam.Permission{
			Resource: p.GetResource(),
			Action:   iam.Action_ACTION_MANAGE,
		})
	}
	return perms, nil
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
	default:
		return "resource"
	}
}
