package api

import (
	"bytes"
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
)

// bytesBuilder is a small io.Reader-producing text builder for exports.
type bytesBuilder struct{ bytes.Buffer }

func (b *bytesBuilder) printf(format string, args ...any) { fmt.Fprintf(&b.Buffer, format, args...) }

// authActor is the auth.Actor alias used by the handlers' helpers.
type authActor = auth.Actor
