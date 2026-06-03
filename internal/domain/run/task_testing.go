package run

import (
	"fmt"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

type pgNoopInstallTask struct {
	client   agent.Client
	state    *State
	topology *types.TestingTopology
}

func (t *pgNoopInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) == 0 {
		return fmt.Errorf("pg-noop target not provisioned")
	}
	version := "0.1.1"
	if t.topology != nil && t.topology.PgNoop != nil && t.topology.PgNoop.Version != "" {
		version = t.topology.PgNoop.Version
	}
	nc.Log().Info("installing pg-noop on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallPgNoop,
		Config: agent.PgNoopInstallConfig{Version: version},
	})
}

type pgNoopConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.TestingTopology
}

func (t *pgNoopConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) == 0 {
		return fmt.Errorf("pg-noop target not provisioned")
	}
	port := 5432
	workers := 0
	version := "0.1.1"
	if t.topology != nil && t.topology.PgNoop != nil {
		if t.topology.PgNoop.Port > 0 {
			port = t.topology.PgNoop.Port
		}
		workers = t.topology.PgNoop.Workers
		if t.topology.PgNoop.Version != "" {
			version = t.topology.PgNoop.Version
		}
	}
	nc.Log().Info("configuring pg-noop")
	for _, target := range targets {
		if err := t.client.Send(nc, target, agent.Command{
			Action: agent.ActionConfigPgNoop,
			Config: agent.PgNoopConfig{
				Host:    "0.0.0.0",
				Port:    port,
				Workers: workers,
			},
		}); err != nil {
			return err
		}
	}
	if host, _ := t.state.DBEndpoint(); host != "" {
		t.state.SetDBEndpoint(host, port)
	}
	t.state.SetEffectiveConfig("database", map[string]string{
		"kind":    "testing",
		"mode":    string(types.TestingPgNoop),
		"pg-noop": version,
		"port":    fmt.Sprintf("%d", port),
	})
	return nil
}
