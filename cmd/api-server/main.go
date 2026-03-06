package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"

	"github.com/stroppy-io/hatchet-workflow/internal/core/logger"
	"github.com/stroppy-io/hatchet-workflow/internal/domain/api"
)

func main() {
	cfg, err := api.NewConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger.NewFromEnv()

	opts := []v0Client.ClientOpt{
		v0Client.WithLogger(logger.Zerolog()),
		v0Client.WithToken(cfg.HatchetToken),
	}
	if cfg.HatchetHost != "" {
		opts = append(opts, v0Client.WithHostPort(cfg.HatchetHost, cfg.HatchetPort))
	}

	hatchet, err := hatchetLib.NewClient(opts...)
	if err != nil {
		log.Fatalf("Failed to create Hatchet client: %v", err)
	}

	srv := api.NewServer(cfg, hatchet)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("API server starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down API server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
}
