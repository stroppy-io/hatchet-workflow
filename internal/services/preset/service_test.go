package preset

// service_test.go: tests for shared helpers in service.go:
// cloneName, ignoreNotFound, preserveEntity, stampNew.

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

func TestCloneName(t *testing.T) {
	tests := []struct {
		override string
		source   string
		want     string
	}{
		{override: "custom", source: "original", want: "custom"},
		{override: "", source: "original", want: "original (copy)"},
		{override: "", source: "", want: "(copy)"},
		{override: "x", source: "", want: "x"},
	}
	for _, tc := range tests {
		got := cloneName(tc.override, tc.source)
		if got != tc.want {
			t.Errorf("cloneName(%q, %q) = %q, want %q", tc.override, tc.source, got, tc.want)
		}
	}
}

func TestIgnoreNotFound(t *testing.T) {
	t.Run("NilError", func(t *testing.T) {
		if ignoreNotFound(nil) != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("ErrNotFound_ReturnsNil", func(t *testing.T) {
		if ignoreNotFound(derrors.ErrNotFound) != nil {
			t.Error("expected nil for ErrNotFound")
		}
	})

	t.Run("OtherError_Mapped", func(t *testing.T) {
		err := ignoreNotFound(errors.New("unexpected db error"))
		if err == nil {
			t.Fatal("expected error for non-NotFound")
		}
	})

	t.Run("StatusError_PassedThrough", func(t *testing.T) {
		original := status.Error(codes.Internal, "some internal error")
		err := ignoreNotFound(original)
		if status.Code(err) != codes.Internal {
			t.Errorf("expected Internal status to pass through, got %v", err)
		}
	})

	t.Run("DomainNotFoundVariant_ReturnsNil", func(t *testing.T) {
		notFound := derrors.NotFound("preset", "preset not found")
		if ignoreNotFound(notFound) != nil {
			t.Error("expected nil for domain NotFound error variant")
		}
	})
}

func TestPreserveEntity(t *testing.T) {
	d := Deps{}

	existing := &common.Entity{
		Id:       "existing-id",
		TenantId: "t1",
		AuthorId: "original-author",
		Name:     "old-name",
	}
	incoming := &common.Entity{
		Id:          "should-be-ignored",
		TenantId:    "should-be-ignored",
		Name:        "new-name",
		Description: "new-desc",
		AuthorId:    "should-be-ignored",
	}

	result := preserveEntity(existing, incoming, d)

	if result.GetId() != "existing-id" {
		t.Errorf("id should be preserved from existing, got %s", result.GetId())
	}
	if result.GetTenantId() != "t1" {
		t.Errorf("tenant_id should be preserved from existing, got %s", result.GetTenantId())
	}
	if result.GetAuthorId() != "original-author" {
		t.Errorf("author_id should be preserved from existing, got %s", result.GetAuthorId())
	}
	if result.GetName() != "new-name" {
		t.Errorf("name should come from incoming, got %s", result.GetName())
	}
	if result.GetDescription() != "new-desc" {
		t.Errorf("description should come from incoming, got %s", result.GetDescription())
	}
	if result.GetIsFavorite() {
		t.Error("is_favorite must be false (never persisted)")
	}
	if result.GetTimings() == nil {
		t.Fatal("timings must be set")
	}
}

func TestPreserveEntity_NilTimings(t *testing.T) {
	d := Deps{}

	// existing entity with NO timings — created_at falls back to now
	existing := &common.Entity{
		Id:       "id1",
		TenantId: "t1",
		AuthorId: "author",
	}
	incoming := &common.Entity{Name: "new"}

	result := preserveEntity(existing, incoming, d)
	if result.GetTimings().GetCreatedAt() == nil {
		t.Error("created_at should be set even when existing has no timings")
	}
}

func TestStampNew(t *testing.T) {
	d := Deps{}

	e := d.stampNew(nil, "tenant1", "author1")
	if e.GetId() == "" {
		t.Error("stampNew should assign a UUID id")
	}
	if e.GetTenantId() != "tenant1" {
		t.Errorf("expected tenant1, got %s", e.GetTenantId())
	}
	if e.GetAuthorId() != "author1" {
		t.Errorf("expected author1, got %s", e.GetAuthorId())
	}
	if e.GetIsFavorite() {
		t.Error("is_favorite must be false")
	}
	if e.GetTimings() == nil {
		t.Fatal("timings must be set")
	}
	if e.GetTimings().GetCreatedAt() == nil {
		t.Error("created_at must be set")
	}
}

func TestStampNew_ExistingEntity(t *testing.T) {
	d := Deps{}

	existing := &common.Entity{Name: "keep-name"}
	e := d.stampNew(existing, "t1", "a1")

	// The function returns the same pointer populated
	if e == nil {
		t.Fatal("should return non-nil entity")
	}
	if e.GetId() == "" {
		t.Error("id must be assigned even when entity pre-exists")
	}
}
