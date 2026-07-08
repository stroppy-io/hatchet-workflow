package iam

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api"
	iampb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"

	// Blank-imported so its init() registers CatalogService's method
	// descriptors (and their (cloud.v1.iam.auth) annotations) into
	// protoregistry.GlobalFiles — TestCatalog_Grantable_PicksUpCatalogServiceResources
	// below asserts Catalog.Grantable's reflection scan picks them up with
	// zero new code in catalog.go, per spec §8.
	_ "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/catalog"
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

func TestCatalogLabel_ProviderWorkflowResources(t *testing.T) {
	cases := []struct {
		perm *iampb.Permission
		want string
	}{
		{&iampb.Permission{Resource: iampb.Resource_RESOURCE_PROVIDER, Action: iampb.Action_ACTION_READ}, "Read provider"},
		{&iampb.Permission{Resource: iampb.Resource_RESOURCE_WORKFLOW, Action: iampb.Action_ACTION_MANAGE}, "Manage workflow"},
	}
	for _, c := range cases {
		if got := catalogLabel(c.perm); got != c.want {
			t.Errorf("catalogLabel(%v) = %q, want %q", c.perm, got, c.want)
		}
	}
}

// resourceAction is a comparable map key pairing a Permission's Resource and
// Action. The brief's own footnote for this test anticipated a plain
// `type iamValue = int32` alias would not compile (assigning a typed enum
// constant to an int32-alias variable requires an explicit conversion in
// Go) and pre-authorized "replace with two parallel maps if a single
// generic key type is awkward" — this struct key is that replacement: a
// genuinely comparable key with no conversions needed at any call site.
type resourceAction struct {
	resource iampb.Resource
	action   iampb.Action
}

// TestCatalog_Grantable_PicksUpCatalogServiceResources is the spec §8
// end-to-end check: with CatalogService's proto now annotated (per-kind
// all_of on the org RPCs), Catalog.Grantable's reflection scan over every
// registered service in the build must surface RESOURCE_PROVIDER/
// RESOURCE_WORKFLOW's ACTION_CREATE/READ/UPDATE/DELETE/LIST entries plus the
// synthesized ACTION_MANAGE wildcard for each — with zero new code in
// catalog.go beyond Task 3's resourceWord label additions.
func TestCatalog_Grantable_PicksUpCatalogServiceResources(t *testing.T) {
	entries, err := NewCatalog().Grantable(context.Background())
	if err != nil {
		t.Fatalf("Grantable: %v", err)
	}
	want := map[resourceAction]bool{
		{iampb.Resource_RESOURCE_PROVIDER, iampb.Action_ACTION_CREATE}: false,
		{iampb.Resource_RESOURCE_PROVIDER, iampb.Action_ACTION_READ}:   false,
		{iampb.Resource_RESOURCE_PROVIDER, iampb.Action_ACTION_UPDATE}: false,
		{iampb.Resource_RESOURCE_PROVIDER, iampb.Action_ACTION_DELETE}: false,
		{iampb.Resource_RESOURCE_PROVIDER, iampb.Action_ACTION_LIST}:   false,
		{iampb.Resource_RESOURCE_PROVIDER, iampb.Action_ACTION_MANAGE}: false,
		{iampb.Resource_RESOURCE_WORKFLOW, iampb.Action_ACTION_CREATE}: false,
		{iampb.Resource_RESOURCE_WORKFLOW, iampb.Action_ACTION_READ}:   false,
		{iampb.Resource_RESOURCE_WORKFLOW, iampb.Action_ACTION_UPDATE}: false,
		{iampb.Resource_RESOURCE_WORKFLOW, iampb.Action_ACTION_DELETE}: false,
		{iampb.Resource_RESOURCE_WORKFLOW, iampb.Action_ACTION_LIST}:   false,
		{iampb.Resource_RESOURCE_WORKFLOW, iampb.Action_ACTION_MANAGE}: false,
	}
	for _, e := range entries {
		key := resourceAction{e.GetPermission().GetResource(), e.GetPermission().GetAction()}
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("Grantable() missing %+v — CatalogService annotations not picked up", k)
		}
	}
}

type fakePermissionCatalog struct {
	entries []*api.CatalogEntry
}

func (f fakePermissionCatalog) Grantable(context.Context) ([]*api.CatalogEntry, error) {
	return f.entries, nil
}
