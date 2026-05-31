//go:build docker_it

// Package dockerprov integration test. Requires a real docker daemon; run with:
//
//	go test -tags=docker_it ./internal/deploy/dockerprov/...
//
// If docker is not reachable the test skips itself.
package dockerprov

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/topology"
)

func TestDeployTeardown(t *testing.T) {
	const (
		testImage = "alpine:latest"
		runID     = "ittest"
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	d, err := New(testImage)
	if err != nil {
		t.Skipf("docker client unavailable: %v", err)
	}
	defer d.Close()

	// Probe the daemon; skip cleanly if it is not reachable.
	if _, err := d.cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon unreachable: %v", err)
	}

	// Keep the base image alive so it is observably running and gets an IP.
	d.cmd = []string{"sleep", "60"}

	ensureImage(ctx, t, d, testImage)

	topo := &topology.Topology{
		Instances: []*topology.Topology_Instance{
			{
				Id: "node0",
				MachineInfo: &deployment.MachineInfo{
					Cores:    1,
					MemoryGb: 1,
				},
			},
		},
	}

	// Clean up any leftovers from a previous failed run.
	_ = d.Teardown(ctx, runID)

	dep, err := d.Deploy(ctx, topo, runID)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Teardown(context.Background(), runID); err != nil {
			t.Errorf("cleanup Teardown: %v", err)
		}
	})

	if dep.Network != "stroppy-"+runID {
		t.Errorf("unexpected network %q", dep.Network)
	}

	inst, ok := dep.Instances["node0"]
	if !ok {
		t.Fatalf("instance node0 not in result: %#v", dep.Instances)
	}
	if inst.ContainerID == "" {
		t.Errorf("empty container ID")
	}
	if inst.IP == "" {
		t.Errorf("empty IP for deployed container")
	}
	if inst.Name != "stroppy-ittest-node0" {
		t.Errorf("unexpected name %q", inst.Name)
	}

	// Assert the container is actually running.
	info, err := d.cli.ContainerInspect(ctx, inst.ContainerID)
	if err != nil {
		t.Fatalf("inspect deployed container: %v", err)
	}
	if info.State == nil || !info.State.Running {
		t.Fatalf("container not running: state=%+v", info.State)
	}

	// Teardown explicitly and assert the container is gone.
	if err := d.Teardown(ctx, runID); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	if _, err := d.cli.ContainerInspect(ctx, inst.ContainerID); err == nil {
		t.Errorf("container still present after teardown")
	}

	// Teardown is idempotent: a second call must not error.
	if err := d.Teardown(ctx, runID); err != nil {
		t.Errorf("second Teardown not idempotent: %v", err)
	}
}

// ensureImage pulls the test image if it is not already present locally.
func ensureImage(ctx context.Context, t *testing.T, d *Deployer, ref string) {
	t.Helper()

	imgs, err := d.cli.ImageList(ctx, image.ListOptions{})
	if err == nil {
		for _, im := range imgs {
			for _, tag := range im.RepoTags {
				if tag == ref {
					return
				}
			}
		}
	}

	rc, err := d.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		t.Skipf("cannot pull %s: %v", ref, err)
	}
	defer rc.Close()

	// Drain the pull stream so the image is fully available before deploy.
	buf := make([]byte, 4096)
	for {
		if _, err := rc.Read(buf); err != nil {
			break
		}
	}
}
