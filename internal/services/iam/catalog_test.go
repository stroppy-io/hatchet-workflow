package iam

import (
	"context"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestCatalog_Grantable(t *testing.T) {
	c := NewCatalog()
	entries, err := c.Grantable(context.Background())
	if err != nil {
		t.Fatalf("Grantable() error = %v", err)
	}

	if len(entries) == 0 {
		t.Errorf("Grantable() returned no entries")
	}

	// Verify MANAGE wildcards are present for discovered resources
	resources := make(map[iam.Resource]bool)
	hasManage := make(map[iam.Resource]bool)

	for _, entry := range entries {
		p := entry.GetPermission()
		resources[p.GetResource()] = true
		if p.GetAction() == iam.Action_ACTION_MANAGE {
			hasManage[p.GetResource()] = true
		}
	}

	for res := range resources {
		if !hasManage[res] {
			t.Errorf("Resource %v missing ACTION_MANAGE entry", res)
		}
	}
}

func TestCatalogLabel(t *testing.T) {
	tests := []struct {
		p    *iam.Permission
		want string
	}{
		{&iam.Permission{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_READ}, "Read account"},
		{&iam.Permission{Resource: iam.Resource_RESOURCE_TENANT, Action: iam.Action_ACTION_CREATE}, "Create tenant"},
		{&iam.Permission{Resource: iam.Resource_RESOURCE_ROLE, Action: iam.Action_ACTION_UPDATE}, "Update role"},
		{&iam.Permission{Resource: iam.Resource_RESOURCE_MEMBERSHIP, Action: iam.Action_ACTION_DELETE}, "Delete membership"},
		{&iam.Permission{Resource: iam.Resource_RESOURCE_SETTINGS, Action: iam.Action_ACTION_LIST}, "List settings"},
		{&iam.Permission{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_MANAGE}, "Manage account"},
		{&iam.Permission{Resource: 999, Action: 999}, "Access resource"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := catalogLabel(tt.p); got != tt.want {
				t.Errorf("catalogLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
