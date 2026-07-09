package ide

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// credentialInURL matches "scheme://user:pass@" so redact can scrub a
// leaked token out of a git error message before it is wrapped/returned.
var credentialInURL = regexp.MustCompile(`(://[^:/@]*:)[^@/]*@`)

// RemoteURL builds a git-smart-HTTP remote URL for owner/repo on a Gitea
// instance at baseURL, embedding token as HTTP basic-auth so `git
// clone`/`git pull` authenticate without an interactive prompt or a
// separate credential-helper file on disk. Gitea accepts any non-empty
// username with the API token as the password over basic auth for git
// smart-HTTP (verified against the same gitea/gitea:1.23 image
// docker-compose.yaml pins, alongside T1's live Gitea check) — "stroppy-bot"
// here is a fixed placeholder username, never read by Gitea for anything
// but the auth handshake.
//
// The token becomes part of the URL string, which os/exec.Command receives
// as a process argument (visible in `ps`/‌/proc on the host running the
// server, exactly like every other exec.Command invocation with a secret
// argument in this codebase — e.g. terraform's provider credentials) and is
// NEVER logged: worktree.go's callers must not fmt.Sprintf/log this return
// value.
func RemoteURL(baseURL, token, owner, repo string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("ide: parse gitea base url: %w", err)
	}
	if token != "" {
		u.User = url.UserPassword("stroppy-bot", token)
	}
	u.Path = fmt.Sprintf("%s/%s.git", owner, repo)
	if owner == "" {
		// Gitea's own "create under self" convention has no git-clone
		// equivalent — the content APIs resolve owner="" via a whoami call
		// (gitrepo.Client.resolveOwner); worktree materialization needs a
		// real path segment, so an empty owner is a caller programming error,
		// not a runtime condition to route around silently.
		return "", fmt.Errorf("ide: remote url: owner must not be empty (resolve gitrepo.Client's own username first)")
	}
	return u.String(), nil
}

// EnsureWorktree clones remoteURL into dest if dest has no .git directory
// yet, else runs `git pull --ff-only` to bring it up to date. It shells out
// to the system git binary (exec.CommandContext) — internal/gitrepo is a
// pure REST client and intentionally does not do this (see its package
// doc); code-server itself also needs a real .git worktree for its built-in
// git UI/terminal to function, which a REST-API-only content sync could
// never provide.
func EnsureWorktree(ctx context.Context, remoteURL, dest string) (string, error) {
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		if err := runGit(ctx, dest, "pull", "--ff-only"); err != nil {
			return "", fmt.Errorf("ide: pull worktree %q: %w", dest, err)
		}
		return dest, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return "", fmt.Errorf("ide: create worktree parent: %w", err)
	}
	if err := runGit(ctx, "", "clone", remoteURL, dest); err != nil {
		return "", fmt.Errorf("ide: clone worktree %q: %w", dest, err)
	}
	return dest, nil
}

// runGit never logs args verbatim on error beyond what exec already
// includes (the command name/exit status) — CombinedOutput's body can
// contain the remote URL (and therefore the embedded token) when git
// itself echoes it back in a fatal error, so callers must treat a non-nil
// error's message as sensitive and avoid re-logging it at a level that
// reaches shared/aggregated logs. This mirrors the posture
// internal/gitrepo.Client's doc already states for GITEA_TOKEN.
func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w: %s", redactArgs(args), err, redact(string(out)))
	}
	return nil
}

// redactArgs strips a credential-bearing URL argument (clone's second
// argument) down to its path, for safe inclusion in an error message.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if u, err := url.Parse(a); err == nil && u.User != nil {
			u.User = url.UserPassword("stroppy-bot", "***")
			out[i] = u.String()
		}
	}
	return out
}

// redact strips any "user:pass@" credential prefix a git error message
// might echo back from a remote URL it failed to reach.
func redact(s string) string {
	re := credentialInURL
	return re.ReplaceAllString(s, "$1***@")
}
