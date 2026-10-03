package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func readExclude(t *testing.T, repo string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	return string(b)
}

func TestInitWritesConfig(t *testing.T) {
	repo := testutil.NewRepo(t)
	res := Run(t, repo, "init", "-d").MustSucceed(t)
	if !strings.Contains(res.Stdout, ".flowtree.json") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
	data, err := os.ReadFile(filepath.Join(repo, ".flowtree.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"base_dir": "../worktrees"`) {
		t.Fatalf("config = %s", data)
	}
	if strings.Contains(readExclude(t, repo), ".flowtree.json") {
		t.Fatal("non-interactive init must not touch info/exclude")
	}
}

func TestInitFromLinkedWorktreeUsesMain(t *testing.T) {
	repo := testutil.NewRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "wt")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "x", wt)
	Run(t, wt, "init", "-d").MustSucceed(t)
	if _, err := os.Stat(filepath.Join(repo, ".flowtree.json")); err != nil {
		t.Fatal("config should be written to the main worktree")
	}
}

func TestInitParentEnvAndExclude(t *testing.T) {
	repo := testutil.NewRepo(t)
	RunEnv(t, repo, []string{"FLOWTREE_PARENT=/srv/trees"}, "init", "--exclude").MustSucceed(t)
	data, _ := os.ReadFile(filepath.Join(repo, ".flowtree.json"))
	if !strings.Contains(string(data), `"base_dir": "/srv/trees/repo"`) {
		t.Fatalf("config = %s", data)
	}
	if !strings.Contains(readExclude(t, repo), "/.flowtree.json") {
		t.Fatal("--exclude should add the file to info/exclude")
	}
	if out := testutil.Git(t, repo, "status", "--porcelain"); strings.Contains(out, ".flowtree.json") {
		t.Fatalf("file should be ignored, status: %q", out)
	}
}

func TestInitTwiceKeepsFileUnlessForced(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-w", "../a").MustSucceed(t)
	res := Run(t, repo, "init", "-w", "../b").MustSucceed(t)
	if !strings.Contains(res.Stdout, "--force") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
	if data, _ := os.ReadFile(filepath.Join(repo, ".flowtree.json")); !strings.Contains(string(data), "../a") {
		t.Fatalf("file changed without --force: %s", data)
	}
	Run(t, repo, "init", "-w", "../b", "--force").MustSucceed(t)
	if data, _ := os.ReadFile(filepath.Join(repo, ".flowtree.json")); !strings.Contains(string(data), "../b") {
		t.Fatalf("--force did not overwrite: %s", data)
	}
}

func TestInitOutsideRepoFails(t *testing.T) {
	res := Run(t, t.TempDir(), "init", "-d").MustFail(t)
	if !strings.Contains(res.Stderr, "not in a git repository") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}
