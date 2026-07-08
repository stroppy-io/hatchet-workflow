package errors

import (
	stderrors "errors"
	"strings"
)

// IgnoreNotFound returns nil for a not-found error (so cascade/idempotent
// deletes are no-ops) and passes any other error through unchanged.
func IgnoreNotFound(err error) error {
	if stderrors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// IgnoreConflict returns nil for a conflict error (so a racing/duplicate
// create becomes a no-op) and passes any other error through unchanged.
func IgnoreConflict(err error) error {
	if stderrors.Is(err, ErrConflict) {
		return nil
	}
	return err
}

/*
	Domain errors are transport-agnostic on purpose: they carry enough structure
	(kind, machine reason, affected resource/field, metadata) for any transport to
	render a precise response, but they import NOTHING about gRPC/HTTP. The
	kind -> status-code translation and protocol error details live at the
	transport seam (see internal/services/utils.MapErr), so swapping transport
	never touches the domain.
*/

// Kind is the transport-agnostic class of a domain error.
type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindConflict
	KindInvalid
	KindFailedPrecondition
	KindPermissionDenied
	KindUnauthenticated
	KindInternal
)

func (k Kind) String() string {
	switch k {
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindInvalid:
		return "invalid"
	case KindFailedPrecondition:
		return "failed_precondition"
	case KindPermissionDenied:
		return "permission_denied"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// Error is a structured domain error.
type Error struct {
	Kind     Kind
	Reason   string            // stable machine code, e.g. "ACCOUNT_NOT_FOUND"
	Resource string            // affected resource type, e.g. "account"
	Field    string            // offending request field (for KindInvalid)
	Message  string            // human-readable message
	Meta     map[string]string // extra structured context
	wrapped  error
}

func (e *Error) Error() string {
	var b strings.Builder
	if e.Resource != "" {
		b.WriteString(e.Resource)
		b.WriteString(": ")
	}
	if e.Message != "" {
		b.WriteString(e.Message)
	} else {
		b.WriteString(e.Kind.String())
	}
	if e.wrapped != nil {
		b.WriteString(": ")
		b.WriteString(e.wrapped.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.wrapped }

// Is matches by Reason when the target carries one, else by Kind. This lets
// errors.Is(richErr, ErrNotFound) succeed for any not-found error while still
// allowing exact matches on a specific Reason.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	if t.Reason != "" {
		return e.Reason == t.Reason
	}
	return e.Kind == t.Kind
}

// New builds a domain error of the given kind.
func New(kind Kind, reason, msg string) *Error {
	return &Error{Kind: kind, Reason: reason, Message: msg}
}

func (e *Error) WithResource(resource string) *Error {
	e.Resource = resource
	return e
}

func (e *Error) WithField(field string) *Error {
	e.Field = field
	return e
}

func (e *Error) WithMeta(key, value string) *Error {
	if e.Meta == nil {
		e.Meta = make(map[string]string)
	}
	e.Meta[key] = value
	return e
}

func (e *Error) Wrap(err error) *Error {
	e.wrapped = err
	return e
}

// Convenience constructors for the common kinds.
func NotFound(resource, msg string) *Error {
	return New(KindNotFound, "", msg).WithResource(resource)
}

func Conflict(resource, msg string) *Error {
	return New(KindConflict, "", msg).WithResource(resource)
}

func Invalid(field, msg string) *Error {
	return New(KindInvalid, "", msg).WithField(field)
}

func FailedPrecondition(reason, msg string) *Error {
	return New(KindFailedPrecondition, reason, msg)
}

func PermissionDenied(msg string) *Error {
	return New(KindPermissionDenied, "", msg)
}

func Unauthenticated(msg string) *Error {
	return New(KindUnauthenticated, "", msg)
}

func Internal(msg string) *Error {
	return New(KindInternal, "", msg)
}

// Sentinels for errors.Is checks. Treat as read-only: build fresh errors with
// the constructors above rather than mutating these.
var (
	ErrNotFound = &Error{Kind: KindNotFound, Message: "not found"}
	ErrConflict = &Error{Kind: KindConflict, Message: "already exists"}
)
