package iam

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestCatalogManagePermissionsReturnsManageForEveryGrantableResource(t *testing.T) {
	catalog := fakePermissionCatalog{entries: []*api.CatalogEntry{
		{Permission: &iampb.Permission{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_LIST}},
		{Permission: &iampb.Permission{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_MANAGE}},
		{Permission: &iampb.Permission{Resource: iampb.Resource_RESOURCE_TEST_RUN, Action: iampb.Action_ACTION_MANAGE}},
		{Permission: &iampb.Permission{Resource: iampb.Resource_RESOURCE_WIZARD, Action: iampb.Action_ACTION_MANAGE}},
	}}

	perms, err := catalogManagePermissions(context.Background(), catalog)
	if err != nil {
		t.Fatalf("catalog manage permissions: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("permissions len = %d, want 2", len(perms))
	}
	got := map[iampb.Resource]iampb.Action{}
	for _, p := range perms {
		got[p.GetResource()] = p.GetAction()
	}
	for _, res := range []iampb.Resource{
		iampb.Resource_RESOURCE_TEST_RUN,
		iampb.Resource_RESOURCE_WIZARD,
	} {
		if got[res] != iampb.Action_ACTION_MANAGE {
			t.Fatalf("resource %s action = %s, want %s", res, got[res], iampb.Action_ACTION_MANAGE)
		}
	}
}

type fakePermissionCatalog struct {
	entries []*api.CatalogEntry
}

func (f fakePermissionCatalog) Grantable(context.Context) ([]*api.CatalogEntry, error) {
	return f.entries, nil
}
