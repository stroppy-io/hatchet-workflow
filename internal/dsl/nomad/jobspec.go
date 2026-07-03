// Package nomad maps a lowered dslpb.ServiceSpec onto a Nomad job spec
// (github.com/hashicorp/nomad/api.Job): the shape the gateway agent submits
// to the local Nomad API (see internal/agent/nomad_activities.go).
//
// BuildJob emits one task group PER target node rather than a single group
// with Count == len(nodes): Nomad's own scheduler has no notion of "this
// stroppy machine group", so pinning is done with a per-group constraint on
// the `${meta.stroppy_node_id}` client fingerprint attribute instead — every
// deployed VM's Nomad client stamps that meta key at agent bootstrap (see
// internal/domain/agent/bootstrap.go Env, which sets STROPPY_NODE_ID and
// wires it into the client's meta block at provisioning time). One group per
// node keeps that placement 1:1 and deterministic.
//
// Config files referenced by ServiceSpec.Configs are NOT rendered here: their
// contents land on the node filesystem via earlier agent steps (WriteFile/
// FetchFile activities) run before the Nomad job starts, so BuildJob only
// needs the task to mount them (that mounting is expressed as a volume in
// ServiceSpec.Volumes by the DSL lowering stage, not by BuildJob).
package nomad

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"

	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

const (
	dockerDriver = "docker"

	// defaultHealthCheckInterval is used for every generated http check: the
	// DSL's HealthCheck message carries only a timeout, not a poll interval,
	// so BuildJob fills in a sensible default rather than hammering the
	// service or waiting too long between probes.
	defaultHealthCheckInterval = 10 * time.Second

	// defaultDatacenter is the only Nomad datacenter stroppy clusters use;
	// Nomad requires a job to name at least one.
	defaultDatacenter = "dc1"

	// nodeIDMetaAttr is the Nomad client fingerprint attribute every agent
	// stamps with its own machine id at bootstrap (see
	// internal/domain/agent/bootstrap.go).
	nodeIDMetaAttr = "${meta.stroppy_node_id}"

	// serviceProviderNomad selects Nomad's built-in service discovery
	// (rather than Consul) — these clusters do not run Consul.
	serviceProviderNomad = "nomad"
)

// NodeRef identifies one target node for a ServiceSpec's task group.
// PrivateIP is carried through for future addressing needs (e.g. advertising
// a reachable address) but is not consumed by BuildJob today.
type NodeRef struct {
	NodeID    string
	PrivateIP string
}

// BuildJob maps svc onto a Nomad job with one task group per entry in nodes,
// each constrained to run only on that node (see package doc). Group/task
// order matches nodes' order, so two calls with the same inputs produce an
// identical *api.Job.
func BuildJob(svc *dslpb.ServiceSpec, nodes []NodeRef) (*api.Job, error) {
	if svc == nil {
		return nil, fmt.Errorf("nomad: BuildJob: nil ServiceSpec")
	}
	name := svc.GetName()
	if name == "" {
		return nil, fmt.Errorf("nomad: BuildJob: ServiceSpec.name is required")
	}
	if svc.GetImage() == "" {
		return nil, fmt.Errorf("nomad: BuildJob %q: ServiceSpec.image is required", name)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("nomad: BuildJob %q: at least one target node is required", name)
	}

	groups := make([]*api.TaskGroup, 0, len(nodes))
	for _, node := range nodes {
		group, err := buildTaskGroup(svc, node)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}

	job := &api.Job{
		ID:          strPtr(name),
		Name:        strPtr(name),
		Type:        strPtr(api.JobTypeService),
		Datacenters: []string{defaultDatacenter},
		TaskGroups:  groups,
	}
	return job, nil
}

// buildTaskGroup builds the single-task group for svc pinned to node.
func buildTaskGroup(svc *dslpb.ServiceSpec, node NodeRef) (*api.TaskGroup, error) {
	name := svc.GetName()
	if node.NodeID == "" {
		return nil, fmt.Errorf("nomad: BuildJob %q: NodeRef.NodeID is required", name)
	}

	task, err := buildTask(svc)
	if err != nil {
		return nil, err
	}

	return &api.TaskGroup{
		Name:  strPtr(groupName(name, node.NodeID)),
		Count: intPtr(1),
		Constraints: []*api.Constraint{
			api.NewConstraint(nodeIDMetaAttr, "=", node.NodeID),
		},
		Tasks: []*api.Task{task},
	}, nil
}

// buildTask builds the docker task carrying svc's image/network/volumes/env
// and (when set) the Nomad service + http health check.
func buildTask(svc *dslpb.ServiceSpec) (*api.Task, error) {
	name := svc.GetName()

	config := map[string]any{
		"image": svc.GetImage(),
	}
	if network := svc.GetNetwork(); network != "" {
		config["network_mode"] = network
	}
	if volumes := svc.GetVolumes(); len(volumes) > 0 {
		config["volumes"] = append([]string(nil), volumes...)
	}

	task := &api.Task{
		Name:   name,
		Driver: dockerDriver,
		Config: config,
	}
	if env := svc.GetEnv(); len(env) > 0 {
		task.Env = copyEnv(env)
	}

	if health := svc.GetHealth(); health != nil {
		service, err := buildService(name, health)
		if err != nil {
			return nil, fmt.Errorf("nomad: BuildJob %q: %w", name, err)
		}
		task.Services = []*api.Service{service}
	}

	return task, nil
}

// buildService builds the Nomad-native service registration + http check for
// health. health.Http is ":<port>/<path>" (e.g. ":8008/health"); with docker
// network_mode host there is no Nomad-managed port label to reference, so the
// check's port is the raw port number instead — Nomad accepts a literal port
// there precisely for the host-networking case (no bridge network ports to
// label).
func buildService(name string, health *dslpb.HealthCheck) (*api.Service, error) {
	port, path, err := splitHTTPHealth(health.GetHttp())
	if err != nil {
		return nil, err
	}

	timeoutStr := health.GetTimeout()
	if timeoutStr == "" {
		return nil, fmt.Errorf("health.timeout is required")
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("health.timeout %q: %w", timeoutStr, err)
	}

	return &api.Service{
		Name:      name,
		PortLabel: port,
		Provider:  serviceProviderNomad,
		Checks: []api.ServiceCheck{
			{
				Type:      "http",
				Path:      path,
				PortLabel: port,
				Interval:  defaultHealthCheckInterval,
				Timeout:   timeout,
			},
		},
	}, nil
}

// splitHTTPHealth splits a ":<port>/<path>" health.http string (e.g.
// ":8008/health") into its numeric port and its path (path keeps the leading
// slash).
func splitHTTPHealth(http string) (port, path string, err error) {
	if !strings.HasPrefix(http, ":") {
		return "", "", fmt.Errorf("health.http %q: expected \":<port>/<path>\" format", http)
	}
	rest := http[1:]
	idx := strings.Index(rest, "/")
	if idx <= 0 {
		return "", "", fmt.Errorf("health.http %q: expected \":<port>/<path>\" format", http)
	}
	port = rest[:idx]
	path = rest[idx:]
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", fmt.Errorf("health.http %q: invalid port: %w", http, err)
	}
	return port, path, nil
}

func groupName(serviceName, nodeID string) string {
	return serviceName + "-" + nodeID
}

func copyEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
