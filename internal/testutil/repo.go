// Package testutil builds real git repositories for tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var identity = map[string]string{
	"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
	"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
}

// Git runs git in dir and fails the test on error.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_EDITOR=true",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// NewRepo creates <tmp>/repo with one commit on branch "main" and returns its path.
// Worktrees default to <tmp>/worktrees, outside the repo, as in real use.
func NewRepo(t testing.TB) string {
	t.Helper()
	// Keep the user's own git configuration (global flowtree.* settings, signing, hooks) out of tests.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	for k, v := range identity {
		t.Setenv(k, v) // flowtree's own git calls (merges, tags) need an identity too
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	Git(t, repo, "init", "-q", "-b", "main")
	WriteFile(t, filepath.Join(repo, "README.md"), "hello\n")
	Git(t, repo, "add", "-A")
	Git(t, repo, "commit", "-q", "-m", "init")
	return repo
}

// NewRepoWithRemote is NewRepo plus a bare "origin" next to it, with main pushed.
func NewRepoWithRemote(t testing.TB) (repo, remote string) {
	t.Helper()
	repo = NewRepo(t)
	remote = filepath.Join(filepath.Dir(repo), "origin.git")
	Git(t, filepath.Dir(repo), "init", "-q", "--bare", "-b", "main", remote)
	Git(t, repo, "remote", "add", "origin", remote)
	Git(t, repo, "push", "-q", "-u", "origin", "main")
	return repo, remote
}

// WriteFile writes content, creating parent directories.
func WriteFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
