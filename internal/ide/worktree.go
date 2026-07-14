package ide

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// credentialInURL matches "scheme://user:pass@" so redact can scrub a
// leaked token out of a git error message before it is wrapped/returned.
var credentialInURL = regexp.MustCompile(`(://[^:/@]*:)[^@/]*@`)

// RemoteURL builds a CREDENTIAL-FREE git-smart-HTTP remote URL for
// owner/repo on a Gitea instance at baseURL.
//
// The token is deliberately NOT embedded. `git clone <url>` persists its
// argument verbatim into the new repo's .git/config as remote.origin.url,
// and EnsureWorktree's output is bind-mounted into a per-org code-server
// container (see Manager.worktreeDir + the WorktreeVolume mount). An
// embedded token would therefore be readable by any org author via
// `cat .git/config` from the IDE terminal — and since the same Gitea
// service account can write the INSTANCE repo, that is a cross-tenant
// privilege escalation, not merely a credential leak.
//
// Authentication is instead supplied per-invocation via an HTTP header
// (see gitAuthArgs), which git accepts on the command line and never
// writes to disk.
func RemoteURL(baseURL, owner, repo string) (string, error) {
	if owner == "" {
		// Gitea's own "create under self" convention has no git-clone
		// equivalent — the content APIs resolve owner="" via a whoami call
		// (gitrepo.Client.resolveOwner); worktree materialization needs a
		// real path segment, so an empty owner is a caller programming error,
		// not a runtime condition to route around silently.
		return "", fmt.Errorf("ide: remote url: owner must not be empty (resolve gitrepo.Client's own username first)")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("ide: parse gitea base url: %w", err)
	}
	u.User = nil
	u.Path = fmt.Sprintf("%s/%s.git", owner, repo)
	return u.String(), nil
}

// gitAuthArgs returns the `-c http.extraHeader=...` prefix that authenticates
// a single git invocation against Gitea without persisting anything. Gitea
// accepts basic auth with any non-empty username and the API token as the
// password; "stroppy-bot" is a fixed placeholder it never reads.
//
// The header value lands in the process argv (visible in `ps`/proc on the
// server host — the same exposure terraform's provider credentials already
// have in this codebase), but NOT in .git/config, NOT in the reflog, and
// therefore NOT inside the container an org author can reach.
func gitAuthArgs(token string) []string {
	if token == "" {
		return nil
	}
	basic := base64.StdEncoding.EncodeToString([]byte("stroppy-bot:" + token))
	return []string{"-c", "http.extraHeader=Authorization: Basic " + basic}
}

// EnsureWorktree clones remoteURL into dest if dest has no .git directory
// yet, else runs `git pull --ff-only` to bring it up to date. It shells out
// to the system git binary (exec.CommandContext) — internal/gitrepo is a
// pure REST client and intentionally does not do this (see its package
// doc); code-server itself also needs a real .git worktree for its built-in
// git UI/terminal to function, which a REST-API-only content sync could
// never provide.
//
// remoteURL must be credential-free (RemoteURL guarantees this): it is what
// git writes into the worktree's .git/config, and the worktree is mounted
// into a per-org code-server. token authenticates each invocation via a
// command-line header instead, so nothing secret reaches the container.
func EnsureWorktree(ctx context.Context, remoteURL, token, dest string) (string, error) {
	auth := gitAuthArgs(token)
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		if err := runGit(ctx, dest, append(auth, "pull", "--ff-only")...); err != nil {
			return "", fmt.Errorf("ide: pull worktree %q: %w", dest, err)
		}
		if err := chownToEditor(dest); err != nil {
			return "", err
		}
		return dest, nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("ide: create worktree parent: %w", err)
	}
	if err := runGit(ctx, "", append(auth, "clone", remoteURL, dest)...); err != nil {
		return "", fmt.Errorf("ide: clone worktree %q: %w", dest, err)
	}
	if err := chownToEditor(dest); err != nil {
		return "", err
	}
	return dest, nil
}

// editorUID/editorGID are the uid:gid code-server runs as inside its image
// (the "coder" user). git runs here as the server process (root in the
// deployed container), so everything it writes is root-owned and mode 0700 —
// code-server could not even list the folder, and the editor opened on
// "Workspace does not exist" while looking perfectly healthy from the outside.
const (
	editorUID = 1000
	editorGID = 1000
)

// chownToEditor hands a materialized worktree to the editor's user. The whole
// tree, not just the root: the author has to be able to edit the files and let
// code-server's own git integration commit them.
func chownToEditor(root string) error {
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, editorUID, editorGID)
	})
	if err != nil {
		return fmt.Errorf("ide: hand worktree %q to the editor user: %w", root, err)
	}
	// The parent (…/recipes) is created by MkdirAll above and would otherwise
	// stay root-owned, which is enough on its own to make the folder
	// unlistable from inside the container.
	if err := os.Lchown(filepath.Dir(root), editorUID, editorGID); err != nil {
		return fmt.Errorf("ide: hand worktree parent of %q to the editor user: %w", root, err)
	}
	return nil
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

// redactArgs scrubs every credential-bearing argument before an error
// message quotes them: the `http.extraHeader=Authorization: Basic <b64>`
// config arg gitAuthArgs injects, and (defensively) any URL that still
// carries userinfo even though RemoteURL no longer produces one.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if strings.HasPrefix(a, "http.extraHeader=") {
			out[i] = "http.extraHeader=***"
			continue
		}
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
