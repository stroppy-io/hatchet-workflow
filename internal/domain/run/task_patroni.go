package run

import (
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// noopTask is a DAG task that does nothing (used as placeholder when a phase
// is logically replaced by another phase but must still exist in the graph).
type noopTask struct{}

func (t *noopTask) Execute(_ *dag.NodeContext) error { return nil }

// patroniInstallTask installs Patroni on all DB nodes.
type patroniInstallTask struct {
	client agent.Client
	state  *State
}

func (t *patroniInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing patroni on DB nodes")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallPatroni,
		Config: agent.PatroniInstallConfig{},
	})
}

// patroniConfigTask configures Patroni on all DB nodes.
type patroniConfigTask struct {
	client    agent.Client
	state     *State
	version   string
	topology  *types.PostgresTopology
	overrides map[string]string // DatabaseConfig.RenderedConfigOverrides — keys: "patroni.yml"
}

func (t *patroniConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring patroni cluster")

	// Build etcd hosts string from first 3 DB targets (etcd is colocated on DB nodes).
	etcdTargets := targets
	if len(etcdTargets) > 3 {
		etcdTargets = etcdTargets[:3]
	}
	var etcdParts []string
	for _, tgt := range etcdTargets {
		host := tgt.InternalHost
		if host == "" {
			host = tgt.Host
		}
		etcdParts = append(etcdParts, fmt.Sprintf("%s:2379", host))
	}
	etcdHosts := strings.Join(etcdParts, ",")

	pgVersion := t.version
	if pgVersion == "" {
		pgVersion = "16"
	}

	syncMode := t.topology.SyncReplicas > 0

	for i, target := range targets {
		host := target.InternalHost
		if host == "" {
			host = target.Host
		}
		cfg := agent.PatroniClusterConfig{
			Name:         "stroppy-pg",
			NodeName:     fmt.Sprintf("pg%d", i),
			PGVersion:    pgVersion,
			ConnectAddr:  host,
			EtcdHosts:    etcdHosts,
			SyncMode:     syncMode,
			SyncCount:    t.topology.SyncReplicas,
			PGOptions:    t.topology.MasterOptions,
			ConfOverride: t.overrides["patroni.yml"],
		}
		if err := t.client.Send(nc, target, agent.Command{
			Action: agent.ActionConfigPatroni,
			Config: cfg,
		}); err != nil {
			return err
		}
	}
	return nil
}
