package recovery

import (
	"context"
	"time"

	"go.uber.org/zap"

	agentsvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/agent"
	opssvc "github.com/stroppy-io/stroppy-cloud/internal/domain/services/ops"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/services/system"
)

// staleAgentThreshold is how long since the last heartbeat before an agent is
// considered stale during the startup sweep.
const staleAgentThreshold = 60 * time.Second

// Run executes a one-shot startup sweep that fixes up any state left dirty by a
// previous crash. It delegates each step to the appropriate service method so
// that all database access goes through ratel where possible; raw SQL is used
// only in RecomputeDagRunStatuses (see that method for rationale).
func Run(
	ctx context.Context,
	systemSvc *system.Service,
	agentSvc *agentsvc.Service,
	webhookSvc *opssvc.WebhookService,
	log *zap.Logger,
) error {
	n, err := systemSvc.RecoverStuckNodeRuns(ctx)
	if err != nil {
		log.Warn("recovery: RecoverStuckNodeRuns failed", zap.Error(err))
	} else {
		log.Info("recovery: node_runs reset to READY", zap.Int64("rows", n))
	}

	n, err = webhookSvc.RecoverInFlightDeliveries(ctx)
	if err != nil {
		log.Warn("recovery: RecoverInFlightDeliveries failed", zap.Error(err))
	} else {
		log.Info("recovery: webhook_deliveries reset to PENDING", zap.Int64("rows", n))
	}

	n, err = agentSvc.MarkStaleAgents(ctx, staleAgentThreshold)
	if err != nil {
		log.Warn("recovery: MarkStaleAgents failed", zap.Error(err))
	} else {
		log.Info("recovery: agents marked STALE", zap.Int64("rows", n))
	}

	n, err = systemSvc.RecomputeDagRunStatuses(ctx)
	if err != nil {
		log.Warn("recovery: RecomputeDagRunStatuses failed", zap.Error(err))
	} else {
		log.Info("recovery: dag_runs status recomputed", zap.Int64("rows", n))
	}

	return nil
}
