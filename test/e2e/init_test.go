package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestInitStoresNothingForDefaults(t *testing.T) {
	repo := testutil.NewRepo(t)
	res := Run(t, repo, "init", "-d").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Nothing to store") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
	if _, err := os.Stat(filepath.Join(repo, ".flowtree.json")); err == nil {
		t.Fatal("init must not create a file in the working tree")
	}
	if got := testutil.Git(t, repo, "status", "--porcelain"); got != "" {
		t.Fatalf("working tree must stay clean: %q", got)
	}
}

func TestInitStoresBaseDirInGitConfig(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-w", "/srv/trees").MustSucceed(t)
	if got := strings.TrimSpace(testutil.Git(t, repo, "config", "--local", "flowtree.baseDir")); got != "/srv/trees" {
		t.Fatalf("git config flowtree.baseDir = %q", got)
	}
}

func TestInitFromLinkedWorktreeUsesMain(t *testing.T) {
	repo := testutil.NewRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "wt")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "x", wt)
	Run(t, wt, "init", "-w", "../mine").MustSucceed(t)
	if got := strings.TrimSpace(testutil.Git(t, repo, "config", "--local", "flowtree.baseDir")); got != "../mine" {
		t.Fatalf("git config = %q", got)
	}
}

func TestParentEnvIsTheDefaultBaseDir(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-d").MustSucceed(t)
	env := []string{"FLOWTREE_PARENT=/srv/trees"}
	res := RunEnv(t, repo, env, "config", "baseDir").MustSucceed(t)
	if strings.TrimSpace(res.Stdout) != "/srv/trees/repo" {
		t.Fatalf("base dir with FLOWTREE_PARENT = %q", res.Stdout)
	}
	// an explicit setting wins over the environment
	Run(t, repo, "config", "baseDir", "/explicit").MustSucceed(t)
	res = RunEnv(t, repo, env, "config", "baseDir").MustSucceed(t)
	if strings.TrimSpace(res.Stdout) != "/explicit" {
		t.Fatalf("explicit baseDir = %q", res.Stdout)
	}
}

func TestInitTwiceKeepsSettingsUnlessForced(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-w", "../a").MustSucceed(t)
	res := Run(t, repo, "init", "-w", "../b").MustSucceed(t)
	if !strings.Contains(res.Stdout, "--force") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
	if got := strings.TrimSpace(testutil.Git(t, repo, "config", "flowtree.baseDir")); got != "../a" {
		t.Fatalf("settings changed without --force: %q", got)
	}
	Run(t, repo, "init", "-w", "../b", "--force").MustSucceed(t)
	if got := strings.TrimSpace(testutil.Git(t, repo, "config", "flowtree.baseDir")); got != "../b" {
		t.Fatalf("--force did not overwrite: %q", got)
	}
}

func TestInitOutsideRepoFails(t *testing.T) {
	res := Run(t, t.TempDir(), "init", "-d").MustFail(t)
	if !strings.Contains(res.Stderr, "not in a git repository") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}
