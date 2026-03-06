package api

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/api/v1/apiv1connect"
)

func NewServer(cfg *Config, hatchet *hatchetLib.Client) *http.Server {
	jwtMgr := NewJWTManager(cfg.JWTSecret)
	settingsStore := NewSettingsStore(cfg.SettingsPath)

	authHandler := NewAuthHandler(cfg.AdminUser, cfg.AdminPassword, jwtMgr)
	testHandler := NewTestHandler(hatchet, settingsStore)
	topologyHandler := NewTopologyHandler()
	settingsHandler := NewSettingsHandler(settingsStore)
	executionHandler := NewExecutionHandler(hatchet)

	interceptors := connect.WithInterceptors(NewAuthInterceptor(jwtMgr))

	mux := chi.NewRouter()
	mux.Use(middleware.Recoverer)
	mux.Use(middleware.RealIP)
	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Connect-Protocol-Version"},
		ExposedHeaders:   []string{"Grpc-Status", "Grpc-Message"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	mux.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Register ConnectRPC handlers
	path, handler := apiv1connect.NewAuthAPIHandler(authHandler, interceptors)
	mux.Handle(path+"*", handler)

	path, handler = apiv1connect.NewTestAPIHandler(testHandler, interceptors)
	mux.Handle(path+"*", handler)

	path, handler = apiv1connect.NewTopologyAPIHandler(topologyHandler, interceptors)
	mux.Handle(path+"*", handler)

	path, handler = apiv1connect.NewSettingsAPIHandler(settingsHandler, interceptors)
	mux.Handle(path+"*", handler)

	path, handler = apiv1connect.NewExecutionAPIHandler(executionHandler, interceptors)
	mux.Handle(path+"*", handler)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}
}
