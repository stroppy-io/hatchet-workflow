// Package pipelines publishes the pipeline binaries shipped with the
// server into every tenant namespace (§7): `stroppy-run push` and friends
// run as subprocesses with the Graphene context in the environment; the
// registry deduplicates the image, so after the first namespace only the
// manifest is recorded. One server build = one revision; a namespace is
// synced when it carries the running server's revision.
package pipelines

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/build"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
)

// Pipelines are the binaries to push, in order.
var Pipelines = []string{"stroppy-run", "stroppy-suite", "stroppy-provider-verify", "stroppy-quotas"}

// Runner executes one push of one pipeline into a namespace.
type Runner interface {
	Push(ctx context.Context, pipeline, namespace string) error
}

// State of a namespace.
type State struct {
	Namespace string
	Revision  string
	Status    string // pending | synced | behind | failed
	Error     string
	PushedAt  *time.Time
	UpdatedAt time.Time
}

// Store persists the state.
type Store interface {
	All(ctx context.Context) ([]State, error)
	Get(ctx context.Context, namespace string) (State, bool, error)
	Set(ctx context.Context, s State) error
	Delete(ctx context.Context, namespace string) error
}

// Namespaces lists the namespaces that must carry the pipelines.
type Namespaces interface {
	Namespaces(ctx context.Context) ([]string, error)
}

// Pusher is the worker.
type Pusher struct {
	runner   Runner
	store    Store
	ns       Namespaces
	log      *xlog.Logger
	revision string
	mu       sync.Mutex
	busy     map[string]bool
}

// New wires the worker; revision defaults to the build version.
func New(runner Runner, store Store, ns Namespaces, log *xlog.Logger) *Pusher {
	return &Pusher{runner: runner, store: store, ns: ns, log: log, revision: build.Version, busy: map[string]bool{}}
}

// Revision is the expected pipeline revision.
func (p *Pusher) Revision() string { return p.revision }

// SyncAll pushes into every namespace that is not at the revision.
// Returns how many namespaces were pushed.
func (p *Pusher) SyncAll(ctx context.Context, force bool) int {
	namespaces, err := p.ns.Namespaces(ctx)
	if err != nil {
		p.log.Warn("pipelines: namespaces", xlog.Error("error", err))
		return 0
	}
	n := 0
	for _, ns := range namespaces {
		if !force {
			if st, ok, err := p.store.Get(ctx, ns); err == nil && ok && st.Status == "synced" && st.Revision == p.revision {
				continue
			}
		}
		if p.Sync(ctx, ns) {
			n++
		}
	}
	return n
}

// Sync pushes every pipeline into one namespace and records the outcome.
// Returns false when a push of the namespace is already in flight.
func (p *Pusher) Sync(ctx context.Context, namespace string) bool {
	p.mu.Lock()
	if p.busy[namespace] {
		p.mu.Unlock()
		return false
	}
	p.busy[namespace] = true
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.busy, namespace); p.mu.Unlock() }()

	_ = p.store.Set(ctx, State{Namespace: namespace, Revision: p.revision, Status: "pending"}) //nolint:errcheck // best-effort progress
	for _, pipeline := range Pipelines {
		if err := p.runner.Push(ctx, pipeline, namespace); err != nil {
			p.log.Warn("pipelines: push failed", xlog.String("namespace", namespace), xlog.String("pipeline", pipeline), xlog.Error("error", err))
			_ = p.store.Set(ctx, State{Namespace: namespace, Revision: p.revision, Status: "failed", Error: pipeline + ": " + err.Error()}) //nolint:errcheck // reported by status
			return true
		}
	}
	now := time.Now().UTC()
	if err := p.store.Set(ctx, State{Namespace: namespace, Revision: p.revision, Status: "synced", PushedAt: &now}); err != nil {
		p.log.Warn("pipelines: record push", xlog.String("namespace", namespace), xlog.Error("error", err))
	}
	return true
}

// Forget drops the state of a deleted namespace.
func (p *Pusher) Forget(ctx context.Context, namespace string) { _ = p.store.Delete(ctx, namespace) } //nolint:errcheck // best-effort

// States lists every namespace's state against the expected revision: a
// synced namespace at another revision reads as behind.
func (p *Pusher) States(ctx context.Context) ([]State, error) {
	list, err := p.store.All(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Status == "synced" && list[i].Revision != p.revision {
			list[i].Status = "behind"
		}
	}
	return list, nil
}

// Run pushes at start, then re-checks on the interval (new tenants are
// pushed on creation; the loop catches failures and restarts).
func (p *Pusher) Run(ctx context.Context, interval time.Duration) {
	p.SyncAll(ctx, false)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.SyncAll(ctx, false)
		}
	}
}

// ExecRunner runs the shipped binaries: `<dir>/<pipeline> push` with the
// Graphene context in the environment (cliconfig field overrides).
type ExecRunner struct {
	Dir     string
	Cfg     *graphene.Config
	Timeout time.Duration
}

// Push implements Runner.
func (r ExecRunner) Push(ctx context.Context, pipeline, namespace string) error {
	bin := filepath.Join(r.Dir, pipeline)
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("binary %s: %w", bin, err)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, "push")
	cmd.Env = append(os.Environ(),
		"GRAPHENE_ADDRESS="+r.Cfg.Address,
		"GRAPHENE_TOKEN="+r.Cfg.Token,
		"GRAPHENE_NAMESPACE="+namespace,
		fmt.Sprintf("GRAPHENE_INSECURE=%t", r.Cfg.Insecure),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		tail := stderr.String()
		if len(tail) > 2000 {
			tail = tail[len(tail)-2000:]
		}
		return fmt.Errorf("%s push: %w: %s", pipeline, err, tail)
	}
	return nil
}

// ErrNoBinaries reports a missing binaries directory (dev runs without
// the pipelines built).
var ErrNoBinaries = errors.New("pipeline binaries directory missing")

// CheckDir reports whether every binary is present.
func CheckDir(dir string) error {
	if dir == "" {
		return ErrNoBinaries
	}
	for _, p := range Pipelines {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			return fmt.Errorf("%w: %s", ErrNoBinaries, p)
		}
	}
	return nil
}
