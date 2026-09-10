package repositories

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

// infraf wraps a storage failure as an internal domain error: callers see
// "internal", logs see the driver message.
func infraf(format string, args ...any) error {
	return errs.Wrap(errs.CodeInternal, "storage", fmt.Errorf(format, args...))
}
