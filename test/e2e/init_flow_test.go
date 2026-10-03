package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

// localSetting reads one flowtree.* key from the repository's own git config ("" when unset).
func localSetting(t *testing.T, repo, key string) string {
	t.Helper()
	res, err := gitx.Exec(t.Context(), repo, "config", "--local", "--get", "flowtree."+key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res)
}

func TestInitFlowCreatesBranchesAndSettings(t *testing.T) {
	repo := testutil.NewRepo(t) // production branch is "main"
	res := Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	if !strings.Contains(res.Stdout, "develop") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	for key, want := range map[string]string{
		"master": "main", "develop": "develop", "workPrefix": "work/", "releasePrefix": "release/", "hotfixPrefix": "hotfix/",
	} {
		if got := localSetting(t, repo, key); got != want {
			t.Errorf("flowtree.%s = %q, want %q", key, got, want)
		}
	}
	if got := strings.TrimSpace(testutil.Git(t, repo, "branch", "--show-current")); got != "develop" {
		t.Errorf("main worktree on %q, want develop", got)
	}
	if got := testutil.Git(t, repo, "status", "--porcelain"); got != "" {
		t.Errorf("init --flow must not dirty the working tree: %q", got)
	}
}

func TestInitFlowFlags(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "--flow", "-d", "--master", "prod", "--develop", "next", "-p", "feat/", "-t", "v", "--remote", "up").MustSucceed(t)
	for key, want := range map[string]string{"master": "prod", "develop": "next", "workPrefix": "feat/", "versionTagPrefix": "v", "remote": "up"} {
		if got := localSetting(t, repo, key); got != want {
			t.Errorf("flowtree.%s = %q, want %q", key, got, want)
		}
	}
	g := gitx.In(t.Context(), repo)
	if !gitx.BranchExists(g, "prod") || !gitx.BranchExists(g, "next") {
		t.Error("branches not created")
	}
}

func TestInitFlowExtendsExistingSettings(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-d", "-w", "/srv/trees").MustSucceed(t)
	Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	if localSetting(t, repo, "baseDir") != "/srv/trees" || localSetting(t, repo, "develop") != "develop" {
		t.Errorf("flow should be added and baseDir kept:\n%s", testutil.Git(t, repo, "config", "--local", "--list"))
	}
	res := Run(t, repo, "init", "--flow", "-d").MustSucceed(t) // already has flow: no-op
	if !strings.Contains(res.Stdout, "already set up") {
		t.Errorf("stdout = %q", res.Stdout)
	}
}

func TestInitFlowRejectsBadSettings(t *testing.T) {
	repo := testutil.NewRepo(t)
	res := Run(t, repo, "init", "--flow", "-d", "--master", "x", "--develop", "x").MustFail(t)
	if !strings.Contains(res.Stderr, "different") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if localSetting(t, repo, "master") != "" {
		t.Error("invalid settings must not be stored")
	}
}

func TestInitFlowNeedsACommit(t *testing.T) {
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	res := Run(t, dir, "init", "--flow", "-d").MustFail(t)
	if !strings.Contains(res.Stderr, "no commits") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if localSetting(t, dir, "master") != "" {
		t.Error("a failed init --flow must not store settings")
	}
}

func TestSettingsLiveInTheMainWorktreesGitConfig(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	// from a linked worktree the same settings apply
	if res := Run(t, worktreeDir(repo, "work/a"), "work", "list").MustSucceed(t); !strings.Contains(res.Stdout, "a") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if _, err := os.Stat(filepath.Join(repo, ".flowtree.json")); err == nil {
		t.Error("no .flowtree.json is created")
	}
}
