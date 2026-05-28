package utils

import (
	"errors"
	"maps"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
)

// errorDomain identifies this service in google.rpc.ErrorInfo.domain.
const errorDomain = "stroppy.io"

// MapErr is the transport seam: it translates a structured domain error into a
// gRPC status error, mapping Kind -> code and attaching google.rpc details
// (ErrorInfo always; BadRequest for field-level validation). Non-domain errors
// collapse to Internal so internals never leak.
func MapErr(err error) error {
	if err == nil {
		return nil
	}
	var de *derrors.Error
	if !errors.As(err, &de) {
		return status.Error(codes.Internal, err.Error())
	}
	st := status.New(kindToCode(de.Kind), de.Error())
	if st2, attachErr := st.WithDetails(errorDetails(de)...); attachErr == nil {
		st = st2
	}
	return st.Err()
}

func kindToCode(kind derrors.Kind) codes.Code {
	switch kind {
	case derrors.KindNotFound:
		return codes.NotFound
	case derrors.KindConflict:
		return codes.AlreadyExists
	case derrors.KindInvalid:
		return codes.InvalidArgument
	case derrors.KindFailedPrecondition:
		return codes.FailedPrecondition
	case derrors.KindPermissionDenied:
		return codes.PermissionDenied
	case derrors.KindUnauthenticated:
		return codes.Unauthenticated
	default:
		return codes.Internal
	}
}

func errorDetails(de *derrors.Error) []protoadapt.MessageV1 {
	reason := de.Reason
	if reason == "" {
		reason = strings.ToUpper(de.Kind.String())
	}
	meta := make(map[string]string, len(de.Meta)+1)
	maps.Copy(meta, de.Meta)
	if de.Resource != "" {
		meta["resource"] = de.Resource
	}
	details := []protoadapt.MessageV1{
		&errdetails.ErrorInfo{Reason: reason, Domain: errorDomain, Metadata: meta},
	}
	if de.Kind == derrors.KindInvalid && de.Field != "" {
		details = append(details, &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{Field: de.Field, Description: de.Message},
			},
		})
	}
	return details
}
