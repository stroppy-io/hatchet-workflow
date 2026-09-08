package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/graphene-ci/pipeline/pkg/machine"
	"github.com/graphene-ci/pipeline/pkg/obs"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// NameHostPrep is the wire name of HostPrep.
const NameHostPrep = "stroppy.host.prep"

// errTailBytes is how much of a failing script's output travels back in the
// error — the whole stream goes to obs, the error carries the tail.
const errTailBytes = 4096

// HostPrepRequest is one host_prep step for the machine this body runs on.
type HostPrepRequest struct {
	Kind    spec.HostPrepKind `json:"kind"`
	Content string            `json:"content"`
	// Index disambiguates several steps of one kind on one role.
	Index int `json:"index"`
}

// HostPrepResult is the step's outcome.
type HostPrepResult struct {
	Output string `json:"output,omitempty"`
}

// HostPrep runs ON THE MACHINE (chrooted from the agent's container):
// sysctl writes a drop-in and reloads; disks and script run the content as
// a shell script. Idempotent by construction of the content (sysctl
// re-applies, the disks script from cfg.host.disks guards mkfs on an
// existing filesystem, user scripts are the user's promise).
func HostPrep(ctx context.Context, req HostPrepRequest) (HostPrepResult, error) {
	switch req.Kind {
	case spec.HostPrepSysctl:
		path := fmt.Sprintf("/etc/sysctl.d/90-stroppy-%d.conf", req.Index)
		if err := os.MkdirAll(filepath.Dir(machine.Path(path)), 0o755); err != nil {
			return HostPrepResult{}, err
		}
		if err := os.WriteFile(machine.Path(path), []byte(req.Content), 0o600); err != nil {
			return HostPrepResult{}, err
		}
		out, err := obs.RunTail(ctx, machine.Shell(ctx, "sysctl --system"), errTailBytes)
		if err != nil {
			return HostPrepResult{}, fmt.Errorf("sysctl --system: %w: %s", err, out)
		}
		return HostPrepResult{Output: out}, nil
	case spec.HostPrepDisks, spec.HostPrepScript:
		out, err := obs.RunTail(ctx, machine.Shell(ctx, "set -e\n"+req.Content), errTailBytes)
		if err != nil {
			return HostPrepResult{}, fmt.Errorf("host prep %s: %w: %s", req.Kind, err, out)
		}
		return HostPrepResult{Output: out}, nil
	default:
		return HostPrepResult{}, fmt.Errorf("host prep: unknown kind %q", req.Kind)
	}
}
