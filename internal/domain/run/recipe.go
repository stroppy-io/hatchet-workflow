package run

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/agent"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// This file holds the primitive-command constructors and dispatch helpers the
// run tasks use to drive the (now dumb) agent. The agent understands only
// run_cmd / write_file / start_daemon / shutdown; every task composes the
// domain-specific behaviour here on the server and ships opaque primitives.

// runCmd builds a run_cmd primitive that executes an opaque bash script.
func runCmd(label, script string) agent.Command {
	return agent.Command{
		Action: agent.ActionRunCmd,
		Label:  label,
		Config: agent.RunCmdConfig{Script: script},
	}
}

// aptCmd builds a run_cmd primitive flagged exclusive so it serializes against
// other apt/dpkg commands on the same agent (dpkg lock contention).
func aptCmd(label, script string) agent.Command {
	return agent.Command{
		Action: agent.ActionRunCmd,
		Label:  label,
		Config: agent.RunCmdConfig{Script: script, Exclusive: true},
	}
}

// writeFile builds a write_file primitive (mode 0644, truncating).
func writeFile(label, path, content string) agent.Command {
	return agent.Command{
		Action: agent.ActionWriteFile,
		Label:  label,
		Config: agent.WriteFileConfig{Path: path, Content: content},
	}
}

// appendFile builds a write_file primitive that appends to an existing file.
func appendFile(label, path, content string) agent.Command {
	return agent.Command{
		Action: agent.ActionWriteFile,
		Label:  label,
		Config: agent.WriteFileConfig{Path: path, Content: content, Append: true},
	}
}

// startDaemonCmd builds a start_daemon primitive (tracked background process).
func startDaemonCmd(label, name, bin string, args []string, env map[string]string) agent.Command {
	return agent.Command{
		Action: agent.ActionStartDaemon,
		Label:  label,
		Config: agent.StartDaemonConfig{Name: name, Bin: bin, Args: args, Env: env},
	}
}

// sendSeq dispatches commands to a single target in order, stopping at the
// first error. Each Send blocks until the agent reports completion.
func sendSeq(nc *NodeContext, client CommandSink, target agent.Target, cmds ...agent.Command) error {
	for _, c := range cmds {
		if len(cmds) == 0 {
			continue
		}
		if err := client.Send(nc, target, c); err != nil {
			return err
		}
	}
	return nil
}

// sendSeqAll dispatches the same command sequence to every target in parallel,
// returning the first error (fail-fast, cancelling the rest).
func sendSeqAll(nc *NodeContext, client CommandSink, targets []agent.Target, cmds ...agent.Command) error {
	if len(targets) == 0 {
		return nil
	}
	if len(targets) == 1 {
		return sendSeq(nc, client, targets[0], cmds...)
	}

	ctx, cancel := context.WithCancel(nc)
	defer cancel()
	childNC := nc.WithContext(ctx)

	var (
		once     sync.Once
		firstErr error
		wg       sync.WaitGroup
	)
	for _, t := range targets {
		wg.Add(1)
		go func(target agent.Target) {
			defer wg.Done()
			if err := sendSeq(childNC, client, target, cmds...); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(t)
	}
	wg.Wait()
	return firstErr
}

// sendPerTarget dispatches a DIFFERENT command sequence per target in parallel,
// returning the first error (fail-fast). build(target) returns the commands for
// that target; an empty/nil result skips the target.
func sendPerTarget(nc *NodeContext, client CommandSink, targets []agent.Target, build func(agent.Target) []agent.Command) error {
	if len(targets) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(nc)
	defer cancel()
	childNC := nc.WithContext(ctx)

	var (
		once     sync.Once
		firstErr error
		wg       sync.WaitGroup
	)
	for _, t := range targets {
		cmds := build(t)
		if len(cmds) == 0 {
			continue
		}
		wg.Add(1)
		go func(target agent.Target, cmds []agent.Command) {
			defer wg.Done()
			if err := sendSeq(childNC, client, target, cmds...); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(t, cmds)
	}
	wg.Wait()
	return firstErr
}

// curlOpts are the retry/timeout flags every binary download uses. github SSL
// handshakes from Yandex Cloud are flaky, so retries matter.
const curlOpts = `--connect-timeout 20 --max-time 300 --retry 3 --retry-delay 5 --retry-connrefused --retry-max-time 600`

// bootstrapScript installs the base utilities every machine needs before any
// apt/curl work. Ported from the old agent bootstrap(). Idempotent and safe to
// run more than once; serialized via the exclusive apt lock.
//
// NOTE: deliberately NO `set -e` — the prep commands (policy-rc.d, systemctl
// stop/mask of a possibly-absent unattended-upgrades, fuser) routinely exit
// non-zero on a clean container and must be tolerated. Only the final apt-get
// determines the script's exit status, so a real package failure still surfaces.
const bootstrapScript = `printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d && chmod +x /usr/sbin/policy-rc.d || true
printf 'DPkg::Lock::Timeout "600";\n' > /etc/apt/apt.conf.d/99stroppy-lock-timeout || true
systemctl stop unattended-upgrades 2>/dev/null || true
systemctl mask unattended-upgrades 2>/dev/null || true
for f in /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock; do fuser -k "$f" 2>/dev/null || true; done
for i in $(seq 1 30); do fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || break; sleep 2; done
apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends curl wget ca-certificates gnupg lsb-release sudo tar gzip python3-pip`

// bootstrapCmd returns the single exclusive command that prepares a machine.
func bootstrapCmd() agent.Command {
	return aptCmd("bootstrap", bootstrapScript)
}

// bootstrapTask runs the base-package bootstrap on every provisioned machine.
// All install phases depend on it, so apt/curl primitives downstream can assume
// curl/wget/etc. are present.
type bootstrapTask struct {
	client CommandSink
	state  *State
}

func (t *bootstrapTask) Execute(nc *NodeContext) error {
	targets := t.state.AllTargets()
	nc.Log().Info("bootstrapping base packages on all machines")
	return sendSeqAll(nc, t.client, targets, bootstrapCmd())
}

// installPackageScript builds the bash that installs a types.Package on a
// machine. Ported verbatim from the old agent installPackage(): custom repo +
// GPG key, pre-install commands, optional .deb download, then apt packages.
// Returned as a single exclusive command so it serializes on the apt lock.
func installPackageScript(pkg types.Package) string {
	var b strings.Builder
	b.WriteString("set -e\n")

	// 1. Custom repo + key.
	if pkg.CustomRepo != "" {
		if pkg.CustomRepoKey != "" {
			fmt.Fprintf(&b,
				`curl -fsSL %q | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/custom.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/custom.gpg`+"\n",
				pkg.CustomRepoKey)
		}
		fmt.Fprintf(&b, `echo %q > /etc/apt/sources.list.d/custom.list && apt-get update`+"\n", pkg.CustomRepo)
	}

	// 2. Pre-install commands.
	for _, cmd := range pkg.PreInstall {
		b.WriteString(cmd)
		b.WriteString("\n")
	}

	// 3. .deb file (server has already replaced DebFilename with a download URL).
	if pkg.DebFilename != "" {
		curlAuth := ""
		if pkg.DebToken != "" {
			curlAuth = fmt.Sprintf(` -H "Authorization: Bearer %s"`, pkg.DebToken)
		}
		fmt.Fprintf(&b,
			`curl -fsSL%s %q -o /tmp/custom_package.deb && DEBIAN_FRONTEND=noninteractive apt-get install -y /tmp/custom_package.deb`+"\n",
			curlAuth, pkg.DebFilename)
	}

	// 4. apt packages.
	if len(pkg.AptPackages) > 0 {
		fmt.Fprintf(&b, `DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s`+"\n",
			strings.Join(pkg.AptPackages, " "))
	}

	return b.String()
}

// advertiseHost returns the address other nodes use to reach this target —
// the internal host (container name / internal IP) with a fallback to Host.
// Replaces the agent-side os.Hostname()/$HOSTNAME resolution.
func advertiseHost(target agent.Target) string {
	if target.InternalHost != "" {
		return target.InternalHost
	}
	return target.Host
}
