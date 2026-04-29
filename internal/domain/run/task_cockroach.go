package run

import (
	"sync"

	"github.com/stroppy-io/stroppy-cloud/internal/core/dag"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// CockroachDB is a homogeneous-node distributed SQL DB:
//   - install: pull tarball from binaries.cockroachdb.com on every node
//   - configure: start each node with --advertise-addr and --join pointing
//                at all other nodes
//   - init: run `cockroach init` once on the first node, then apply any
//           SET CLUSTER SETTING from topology.Options
//
// Three small tasks, mirroring the YDB shape (install / configure / init)
// but without the storage-vs-compute role split — every node is the same.

type cockroachInstallTask struct {
	client  agent.Client
	state   *State
	version string
}

func (t *cockroachInstallTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("installing cockroach on targets")
	return t.client.SendAll(nc, targets, agent.Command{
		Action: agent.ActionInstallCockroach,
		Config: agent.CockroachInstallConfig{Version: t.version},
	})
}

type cockroachConfigTask struct {
	client   agent.Client
	state    *State
	topology *types.CockroachTopology
}

func (t *cockroachConfigTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	nc.Log().Info("configuring cockroach cluster")

	// Build the --join peer list. Use InternalHost (container/internal IP)
	// so cluster traffic stays in-VPC; cockroach addresses peers by the
	// same string we put in --advertise-addr, so they have to match.
	peers := make([]string, len(targets))
	for i, tgt := range targets {
		h := tgt.InternalHost
		if h == "" {
			h = tgt.Host
		}
		peers[i] = h
	}

	// Start every node in parallel — cockroach nodes have to be up
	// concurrently to discover each other via --join. Init runs after.
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for i, target := range targets {
		advHost := target.InternalHost
		if advHost == "" {
			advHost = target.Host
		}
		cfg := agent.CockroachClusterConfig{
			NodeIndex:     i,
			AdvertiseHost: advHost,
			Peers:         peers,
			MemoryMB:      t.topology.Nodes.MemoryMB,
		}
		wg.Add(1)
		go func(idx int, tgt agent.Target, c agent.CockroachClusterConfig) {
			defer wg.Done()
			errs[idx] = t.client.Send(nc, tgt, agent.Command{
				Action: agent.ActionConfigCockroach, Config: c,
			})
		}(i, target, cfg)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	// DB endpoint is set in task_infra (port 26257 from the protocol
	// registry); nothing else to do here.
	return nil
}

type cockroachInitTask struct {
	client   agent.Client
	state    *State
	topology *types.CockroachTopology
}

func (t *cockroachInitTask) Execute(nc *dag.NodeContext) error {
	targets := t.state.DBTargets()
	if len(targets) == 0 {
		return nil
	}
	first := targets[0]
	host := first.InternalHost
	if host == "" {
		host = first.Host
	}
	nc.Log().Info("initialising cockroach cluster")
	return t.client.Send(nc, first, agent.Command{
		Action: agent.ActionInitCockroach,
		Config: agent.CockroachInitConfig{
			Host:            host,
			Port:            26257,
			ClusterSettings: t.topology.Options,
		},
	})
}
