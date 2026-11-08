package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestFeaturePullAmbiguousNameIsNotMisreportedAsMissing(t *testing.T) {
	repo, _ := flowRepoWithRemote(t)
	testutil.Git(t, repo, "branch", "feature/login", "develop")
	testutil.Git(t, repo, "branch", "feature/logout", "develop")
	res := Run(t, repo, "feature", "pull", "origin", "log").MustFail(t)
	if !strings.Contains(res.Stderr, "multiple branches match") || strings.Contains(res.Stderr, "not found on") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestFeaturePullCreatesAWorktreeForARemoteOnlyBranch(t *testing.T) {
	repo, _ := flowRepoWithRemote(t)
	testutil.Git(t, repo, "push", "-q", "origin", "develop:feature/remote-only")
	Run(t, repo, "feature", "pull", "origin", "remote-only").MustSucceed(t)
	if !exists(worktreeDir(repo, "feature/remote-only")) {
		t.Error("pull should create the missing local branch with a worktree")
	}
	res := Run(t, repo, "feature", "pull", "origin", "nope").MustFail(t)
	if !strings.Contains(res.Stderr, "not found on 'origin'") {
		t.Errorf("a name that exists nowhere keeps the not-found message: %q", res.Stderr)
	}
}

func TestAddFailingExecKeepsPathAndCdTarget(t *testing.T) {
	repo := testutil.NewRepo(t)
	cdfile := filepath.Join(t.TempDir(), "cd")
	res := RunEnv(t, repo, []string{"FLOWTREE_CDFILE=" + cdfile}, "add", "-b", "feat", "--quiet", "--exec", "exit 7").MustFail(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees", "feat")
	if strings.TrimSpace(res.Stdout) != wt {
		t.Errorf("stdout = %q, want the path %s", res.Stdout, wt)
	}
	if b, _ := os.ReadFile(cdfile); string(b) != wt {
		t.Errorf("cd-file = %q", b)
	}
	if !strings.Contains(res.Stderr, "--exec command failed") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}
