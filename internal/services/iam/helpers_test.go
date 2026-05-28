package iam

import (
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/iam"
)

func TestHelpers_SelfOrAdmin(t *testing.T) {
	if !selfOrAdmin(&iam.AccessClaims{IsAdmin: true}, "a2") {
		t.Error("admin should be true")
	}
	if !selfOrAdmin(&iam.AccessClaims{AccountId: "a1"}, "a1") {
		t.Error("self should be true")
	}
	if selfOrAdmin(&iam.AccessClaims{AccountId: "a1"}, "a2") {
		t.Error("other should be false")
	}
}

func TestHelpers_HasAll(t *testing.T) {
	granted := []*iam.Permission{
		{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_READ},
		{Resource: iam.Resource_RESOURCE_TENANT, Action: iam.Action_ACTION_MANAGE},
	}
	required := []*iam.Permission{
		{Resource: iam.Resource_RESOURCE_TENANT, Action: iam.Action_ACTION_CREATE},
	}
	if !hasAll(granted, required) {
		t.Error("manage should satisfy create")
	}

	required2 := []*iam.Permission{
		{Resource: iam.Resource_RESOURCE_ACCOUNT, Action: iam.Action_ACTION_UPDATE},
	}
	if hasAll(granted, required2) {
		t.Error("read should not satisfy update")
	}
}

func TestHelpers_EmailDomain(t *testing.T) {
	if emailDomain("user@example.com") != "example.com" {
		t.Error("expected example.com")
	}
	if emailDomain("invalid") != "" {
		t.Error("expected empty")
	}
}

func TestHelpers_DomainAllowed(t *testing.T) {
	if !domainAllowed("u@ex.com", []string{}) {
		t.Error("empty allow list should allow all")
	}
	if !domainAllowed("u@Ex.Com", []string{"ex.com"}) {
		t.Error("case insensitive allowed")
	}
	if domainAllowed("u@other.com", []string{"ex.com"}) {
		t.Error("should deny other")
	}
}

func TestHelpers_SanitizeNickname(t *testing.T) {
	if sanitizeNickname("John.Doe+tag@example.com", "fallback") != "John.Doetag" {
		t.Errorf("got %s", sanitizeNickname("John.Doe+tag@example.com", "fallback"))
	}
	if sanitizeNickname("a@", "fallback") != "user-fallback" {
		t.Error("too short should use fallback")
	}
}
