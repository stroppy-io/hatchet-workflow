package application

import (
	"context"
	"fmt"

	"github.com/gopherex/xlog"
	"github.com/gopherex/xprobe"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

/*
INFRA: connections come up in order and the first failure aborts — a
process that started without its database would answer clients with
errors while looking alive. Graphene is dialed lazily: its probe gates
readiness, not startup, so a Graphene redeploy does not crash-loop us.
*/

// Infra is the raised connections.
type Infra struct {
	Postgres *postgres.Client
	Graphene *graphene.Client
}

func connectInfra(ctx context.Context, cfg *InfraConfig, log *xlog.Logger) (*Infra, error) {
	pg, err := postgres.New(ctx, &cfg.Postgres, log)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	log.Info("infra connected", xlog.String("service", "postgres"))
	return &Infra{Postgres: pg, Graphene: graphene.New(&cfg.Graphene)}, nil
}

// Probes are the named dependency probes (readiness and /public/health).
func (in *Infra) Probes() map[string]xprobe.Probe {
	return map[string]xprobe.Probe{
		"postgres": in.Postgres.Probe(),
		"graphene": in.Graphene.Probe(),
	}
}

// Probe is process readiness: every dependency at once.
func (in *Infra) Probe() xprobe.Probe {
	return xprobe.All(in.Postgres.Probe(), in.Graphene.Probe())
}

// Close releases what holds state.
func (in *Infra) Close() {
	if in.Postgres != nil {
		in.Postgres.Close()
	}
}

// namespaces adapts the Graphene client to the tenant port.
type namespaces struct{ c *graphene.Client }

func (n namespaces) EnsureNamespace(ctx context.Context, name string, labels map[string]string) error {
	return n.c.EnsureNamespace(ctx, name, graphene.NamespaceSpec{Description: "stroppy tenant " + name}, labels)
}

func (n namespaces) DeleteNamespace(ctx context.Context, name string) error {
	return n.c.DeleteNamespace(ctx, name)
}
