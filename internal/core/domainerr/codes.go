package domainerr

import (
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

type codefulErr struct{ code errorspb.Code }

func (c *codefulErr) Error() string { return c.code.String() }

func Codeful(code errorspb.Code) error { return &codefulErr{code: code} }

func NotFound(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_CODE_NOT_FOUND, details...)
}

func AlreadyExists(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_CODE_ALREADY_EXISTS, details...)
}

func PermissionDenied(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_CODE_PERMISSION_DENIED, details...)
}

func Unauthenticated(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_CODE_UNAUTHENTICATED, details...)
}

func InvalidArgument(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_CODE_INVALID_ARGUMENT, details...)
}

func FailedPrecondition(details ...*errorspb.Detail) *Error {
	return E(errorspb.Code_CODE_FAILED_PRECONDITION, details...)
}

func ResourceInfo(resourceType, name string) *errorspb.Detail {
	return &errorspb.Detail{Kind: &errorspb.Detail_ResourceInfo{
		ResourceInfo: &errorspb.ResourceInfo{ResourceType: resourceType, ResourceName: name},
	}}
}

func ErrorInfo(reason, domain string, metadata map[string]string) *errorspb.Detail {
	return &errorspb.Detail{Kind: &errorspb.Detail_ErrorInfo{
		ErrorInfo: &errorspb.ErrorInfo{Reason: reason, Domain: domain, Metadata: metadata},
	}}
}

func FieldViolation(field, description string) *errorspb.Detail {
	return &errorspb.Detail{Kind: &errorspb.Detail_FieldViolation{
		FieldViolation: &errorspb.FieldViolation{Field: field, Description: description},
	}}
}
