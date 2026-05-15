package domainerr_test

import (
	"errors"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

func TestErrorCarriesCode(t *testing.T) {
	e := domainerr.E(errorspb.Code_CODE_NOT_FOUND)
	if e.Code() != errorspb.Code_CODE_NOT_FOUND {
		t.Fatalf("expected CODE_NOT_FOUND, got %v", e.Code())
	}
	if e.Error() != "CODE_NOT_FOUND" {
		t.Fatalf("Error() should return enum name, got %q", e.Error())
	}
}

func TestErrorsIsMatchesCode(t *testing.T) {
	e := domainerr.E(errorspb.Code_CODE_PERMISSION_DENIED)
	if !errors.Is(e, domainerr.Codeful(errorspb.Code_CODE_PERMISSION_DENIED)) {
		t.Fatal("errors.Is should match same code")
	}
	if errors.Is(e, domainerr.Codeful(errorspb.Code_CODE_NOT_FOUND)) {
		t.Fatal("errors.Is should not match different code")
	}
}

func TestErrorDetailsAttached(t *testing.T) {
	d := domainerr.ResourceInfo("user", "u-1")
	e := domainerr.E(errorspb.Code_CODE_NOT_FOUND, d)
	if len(e.Details()) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(e.Details()))
	}
}
