package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

func readConfig(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, ".flowtree.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitFlowCreatesBranchesAndSettings(t *testing.T) {
	repo := testutil.NewRepo(t) // production branch is "main"
	res := Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	if !strings.Contains(res.Stdout, "develop") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	cfg := readConfig(t, repo)
	for _, want := range []string{`"master": "main"`, `"develop": "develop"`, `"work_prefix": "work/"`, `"release_prefix": "release/"`, `"hotfix_prefix": "hotfix/"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config lacks %s:\n%s", want, cfg)
		}
	}
	if got := strings.TrimSpace(testutil.Git(t, repo, "branch", "--show-current")); got != "develop" {
		t.Errorf("main worktree on %q, want develop", got)
	}
}

func TestInitFlowFlags(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "--flow", "-d", "--master", "prod", "--develop", "next", "-p", "feat/", "-t", "v", "--remote", "up").MustSucceed(t)
	cfg := readConfig(t, repo)
	for _, want := range []string{`"master": "prod"`, `"develop": "next"`, `"work_prefix": "feat/"`, `"version_tag_prefix": "v"`, `"remote": "up"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config lacks %s:\n%s", want, cfg)
		}
	}
	g := gitx.In(t.Context(), repo)
	if !gitx.BranchExists(g, "prod") || !gitx.BranchExists(g, "next") {
		t.Error("branches not created")
	}
}

func TestInitFlowExtendsExistingConfig(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-d", "-w", "/srv/trees").MustSucceed(t)
	Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	cfg := readConfig(t, repo)
	if !strings.Contains(cfg, `"base_dir": "/srv/trees"`) || !strings.Contains(cfg, `"develop"`) {
		t.Errorf("flow should be added and base_dir kept:\n%s", cfg)
	}
	res := Run(t, repo, "init", "--flow", "-d").MustSucceed(t) // already has flow: no-op
	if !strings.Contains(res.Stdout, "already has flow") {
		t.Errorf("stdout = %q", res.Stdout)
	}
}

func TestInitFlowRejectsBadSettings(t *testing.T) {
	repo := testutil.NewRepo(t)
	res := Run(t, repo, "init", "--flow", "-d", "--master", "x", "--develop", "x").MustFail(t)
	if !strings.Contains(res.Stderr, "different") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestInitFlowNeedsACommit(t *testing.T) {
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	res := Run(t, dir, "init", "--flow", "-d").MustFail(t)
	if !strings.Contains(res.Stderr, "no commits") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestInitFlowFailureWritesNothing(t *testing.T) {
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	Run(t, dir, "init", "--flow", "-d").MustFail(t)
	if _, err := os.Stat(filepath.Join(dir, ".flowtree.json")); err == nil {
		t.Error("a failed init --flow must not leave a config behind")
	}
}
