package ide

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnsureWorktree_ClonesThenPulls(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	// A real local git repo stands in for a Gitea remote — exercises the
	// actual `git clone`/`git pull` shell-out without needing a live Gitea
	// instance, same approach the plan's illustrative test used.
	remote := t.TempDir()
	run(t, remote, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(remote, "cluster.yaml"), []byte("provider:\n  use: docker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, remote, "add", ".")
	run(t, remote, "-c", "user.email=t@t.io", "-c", "user.name=t", "commit", "-m", "init")

	dest := filepath.Join(t.TempDir(), "acme")
	path, err := EnsureWorktree(context.Background(), remote, "", dest)
	if err != nil {
		t.Fatalf("ensure worktree (clone): %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "cluster.yaml")); err != nil {
		t.Fatalf("cloned file missing: %v", err)
	}

	// A second commit on the remote must be picked up by a pull, not require
	// a fresh clone.
	if err := os.WriteFile(filepath.Join(remote, "workflow.yaml"), []byte("name: oltp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, remote, "add", ".")
	run(t, remote, "-c", "user.email=t@t.io", "-c", "user.name=t", "commit", "-m", "add workflow")

	path2, err := EnsureWorktree(context.Background(), remote, "", dest)
	if err != nil {
		t.Fatalf("ensure worktree (pull): %v", err)
	}
	if path2 != path {
		t.Fatalf("path changed between calls: %q vs %q", path, path2)
	}
	if _, err := os.Stat(filepath.Join(dest, "workflow.yaml")); err != nil {
		t.Fatalf("pulled file missing: %v", err)
	}
}

func TestRemoteURL_NeverEmbedsCredentials(t *testing.T) {
	// git clone persists its URL argument into .git/config, and that worktree
	// is mounted into a per-org code-server: an embedded token would be
	// readable by any org author and would grant instance-repo write.
	u, err := RemoteURL("http://gitea:3000", "acme", "org-catalog")
	if err != nil {
		t.Fatalf("remote url: %v", err)
	}
	want := "http://gitea:3000/acme/org-catalog.git"
	if u != want {
		t.Fatalf("remote url = %q, want %q", u, want)
	}
}

func TestGitAuthArgs_CarriesTokenOffDisk(t *testing.T) {
	args := gitAuthArgs("sekret-token")
	if len(args) != 2 || args[0] != "-c" {
		t.Fatalf("unexpected auth args: %v", args)
	}
	if !contains(args[1], base64.StdEncoding.EncodeToString([]byte("stroppy-bot:sekret-token"))) {
		t.Fatalf("auth header missing encoded credential: %q", args[1])
	}
	if gitAuthArgs("") != nil {
		t.Fatal("empty token must yield no auth args")
	}
}

func TestRedactArgs_ScrubsAuthHeader(t *testing.T) {
	for _, a := range redactArgs(gitAuthArgs("sekret-token")) {
		if contains(a, "sekret-token") {
			t.Fatalf("token leaked through redactArgs: %q", a)
		}
	}
}

func TestRemoteURL_RejectsEmptyOwner(t *testing.T) {
	if _, err := RemoteURL("http://gitea:3000", "", "instance-catalog"); err == nil {
		t.Fatal("expected error for empty owner")
	}
}

func TestRedact_ScrubsTokenFromGitErrorOutput(t *testing.T) {
	msg := redact("fatal: unable to access 'http://stroppy-bot:sekret-token@gitea:3000/acme/org-catalog.git/': Could not resolve host")
	if want := "sekret-token"; contains(msg, want) {
		t.Fatalf("redacted message still contains the token: %q", msg)
	}
	if !contains(msg, "stroppy-bot") {
		t.Fatalf("redacted message dropped the username too: %q", msg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestEnsureWorktree_TokenNeverLandsInGitConfig is the regression guard for
// the cross-tenant escalation: the worktree is bind-mounted into a per-org
// code-server, so anything git persists under .git/ is readable by an org
// author. A credentialed clone URL would hand them the Gitea service-account
// token, which can write the INSTANCE repo.
func TestEnsureWorktree_TokenNeverLandsInGitConfig(t *testing.T) {
	remote := t.TempDir()
	run(t, remote, "init", "-q", "--initial-branch=main", ".")
	if err := os.WriteFile(filepath.Join(remote, "cluster.yaml"), []byte("provider:\n  use: docker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, remote, "add", ".")
	run(t, remote, "-c", "user.email=t@t.io", "-c", "user.name=t", "commit", "-m", "init")

	dest := filepath.Join(t.TempDir(), "acme")
	if _, err := EnsureWorktree(context.Background(), remote, "sekret-token", dest); err != nil {
		t.Fatalf("ensure worktree: %v", err)
	}
	for _, p := range []string{".git/config", ".git/logs/HEAD", ".git/FETCH_HEAD"} {
		b, err := os.ReadFile(filepath.Join(dest, p))
		if err != nil {
			continue // not every file exists after a local clone
		}
		if contains(string(b), "sekret-token") {
			t.Fatalf("token persisted into %s", p)
		}
	}
}
