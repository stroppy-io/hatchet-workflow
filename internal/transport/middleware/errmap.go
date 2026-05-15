package middleware

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/core/domainerr"
	errorspb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/errors"
)

func ErrorMapper() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			return resp, toConnect(err)
		}
	}
}

func toConnect(err error) error {
	var de *domainerr.Error
	if !errors.As(err, &de) {
		// already a connect.Error or unknown — let it pass through
		var ce *connect.Error
		if errors.As(err, &ce) {
			return ce
		}
		return connect.NewError(connect.CodeInternal, errors.New(errorspb.Code_CODE_INTERNAL.String()))
	}
	cerr := connect.NewError(mapCode(de.Code()), errors.New(de.Code().String()))
	for _, d := range de.Details() {
		detail, derr := connect.NewErrorDetail(d)
		if derr == nil {
			cerr.AddDetail(detail)
		}
	}
	return cerr
}

func mapCode(c errorspb.Code) connect.Code {
	switch c {
	case errorspb.Code_CODE_OK:
		return connect.Code(0)
	case errorspb.Code_CODE_CANCELLED:
		return connect.CodeCanceled
	case errorspb.Code_CODE_INVALID_ARGUMENT:
		return connect.CodeInvalidArgument
	case errorspb.Code_CODE_DEADLINE_EXCEEDED:
		return connect.CodeDeadlineExceeded
	case errorspb.Code_CODE_NOT_FOUND:
		return connect.CodeNotFound
	case errorspb.Code_CODE_ALREADY_EXISTS:
		return connect.CodeAlreadyExists
	case errorspb.Code_CODE_PERMISSION_DENIED:
		return connect.CodePermissionDenied
	case errorspb.Code_CODE_RESOURCE_EXHAUSTED:
		return connect.CodeResourceExhausted
	case errorspb.Code_CODE_FAILED_PRECONDITION:
		return connect.CodeFailedPrecondition
	case errorspb.Code_CODE_ABORTED:
		return connect.CodeAborted
	case errorspb.Code_CODE_OUT_OF_RANGE:
		return connect.CodeOutOfRange
	case errorspb.Code_CODE_UNIMPLEMENTED:
		return connect.CodeUnimplemented
	case errorspb.Code_CODE_UNAVAILABLE:
		return connect.CodeUnavailable
	case errorspb.Code_CODE_DATA_LOSS:
		return connect.CodeDataLoss
	case errorspb.Code_CODE_UNAUTHENTICATED:
		return connect.CodeUnauthenticated
	default:
		return connect.CodeInternal
	}
}
