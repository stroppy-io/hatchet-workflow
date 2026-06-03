package quotas

import (
	"context"
	"log/slog"
	"time"
)

func StartRefresher(ctx context.Context, manager *Manager, interval time.Duration, log *slog.Logger) {
	if manager == nil || interval <= 0 {
		return
	}
	if log == nil {
		log = slog.Default()
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := manager.RefreshAllConfigured(ctx); err != nil {
					log.Warn("quota refresh failed", slog.String("error", err.Error()))
				}
			}
		}
	}()
}
