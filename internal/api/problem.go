package api

import (
	"context"
	"errors"
	"net/url"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/gopherex/xlog"
	ht "github.com/ogen-go/ogen/http"
	"github.com/ogen-go/ogen/ogenerrors"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
)

// validationOf renders a schemapb result on the wire.
func validationOf(vr *schemapb.ValidationResult) oas.ValidationResult {
	out := oas.ValidationResult{Errors: []oas.ValidationError{}}
	for _, e := range vr.GetErrors() {
		ve := oas.ValidationError{Path: e.GetPath(), Code: e.GetCode().String()}
		if m := e.GetMessage(); m != "" {
			ve.Message = oas.NewOptString(m)
		}
		if c := e.GetConstraint(); c != "" {
			ve.Constraint = oas.NewOptString(c)
		}
		if r := e.GetRuleId(); r != "" {
			ve.RuleID = oas.NewOptString(r)
		}
		sev := oas.ValidationErrorSeverityERROR
		if e.GetSeverity() == schemapb.Schema_Field_SEVERITY_WARNING {
			sev = oas.ValidationErrorSeverityWARNING
		}
		ve.Severity = oas.NewOptValidationErrorSeverity(sev)
		out.Errors = append(out.Errors, ve)
	}
	return out
}

// NewError maps any handler error to a Problem. Domain errors keep their
// code; ogen decode/validation errors become "invalid"; everything else is
// an internal error that is logged with its cause and returned bare.
func (h *Handler) NewError(ctx context.Context, err error) *oas.ProblemStatusCode {
	code := errs.CodeInternal
	detail := ""
	var validation oas.OptValidationResult

	var domain *errs.Error
	var decode *ogenerrors.DecodeRequestError
	var params *ogenerrors.DecodeParamsError
	var security *ogenerrors.SecurityError
	switch {
	case errors.As(err, &domain):
		code, detail = domain.Code, domain.Detail
		if vr, ok := domain.Validation.(*schemapb.ValidationResult); ok && vr != nil {
			validation = oas.NewOptValidationResult(validationOf(vr))
		}
	case errors.As(err, &decode):
		code, detail = errs.CodeInvalid, decode.Error()
	case errors.As(err, &params):
		code, detail = errs.CodeInvalid, params.Error()
	case errors.As(err, &security):
		code, detail = errs.CodeUnauthenticated, "authentication required"
	case errors.Is(err, ht.ErrNotImplemented):
		code, detail = errs.CodeInternal, "not implemented"
	}
	if code == errs.CodeInternal {
		h.deps.Log.Ctx().Error(ctx, "request failed", xlog.ErrorCause(err))
	}
	status := errs.Status(code)
	if errors.Is(err, ht.ErrNotImplemented) {
		status = 501
	}
	p := oas.Problem{
		Type:   url.URL{Scheme: "https", Host: "stroppy.io", Path: "/problems/" + string(code)},
		Title:  title(code),
		Status: status,
		Code:   string(code),
	}
	if detail != "" {
		p.Detail = oas.NewOptString(detail)
	}
	if id := middleware.GetReqID(ctx); id != "" {
		p.RequestID = oas.NewOptString(id)
	}
	p.Validation = validation
	return &oas.ProblemStatusCode{StatusCode: status, Response: p}
}

func title(code errs.Code) string {
	switch code {
	case errs.CodeUnauthenticated:
		return "Unauthenticated"
	case errs.CodeForbidden:
		return "Forbidden"
	case errs.CodeNotFound:
		return "Not found"
	case errs.CodeConflict:
		return "Conflict"
	case errs.CodeInvalid:
		return "Invalid request"
	case errs.CodeValidation:
		return "Validation failed"
	case errs.CodeLimit:
		return "Limit exceeded"
	case errs.CodeUnavailable:
		return "Temporarily unavailable"
	case errs.CodeInternal:
		return "Internal error"
	default:
		return "Internal error"
	}
}
