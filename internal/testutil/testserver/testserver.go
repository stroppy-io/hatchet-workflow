package testserver

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/iam"
	transportconnect "github.com/stroppy-io/stroppy-cloud/internal/transport/connect"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/middleware"
)

// ForIAM returns an http.Handler that wires IAM handlers with Auth + ErrorMapper
// middleware, suitable for httptest.NewServer.
func ForIAM(svc *iam.Service) http.Handler {
	interceptors := connect.WithInterceptors(
		middleware.Auth(svc, transportconnect.AuthBypass()),
		middleware.ErrorMapper(),
	)
	return transportconnect.Mount(transportconnect.Deps{
		IAMHandler:   transportconnect.NewIAMHandler(svc),
		Interceptors: interceptors,
	})
}
