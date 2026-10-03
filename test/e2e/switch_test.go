package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestSwitchByShortNameAndBranch(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "login").MustSucceed(t)
	wt := worktreeDir(repo, "work/login")
	for _, q := range []string{"login", "work/login"} {
		cdfile := filepath.Join(t.TempDir(), "cd")
		res := RunEnv(t, repo, []string{"FLOWTREE_CDFILE=" + cdfile}, "switch", q).MustSucceed(t)
		if strings.TrimSpace(res.Stdout) != wt {
			t.Errorf("switch %s printed %q, want %s", q, res.Stdout, wt)
		}
		if cd, _ := os.ReadFile(cdfile); string(cd) != wt {
			t.Errorf("switch %s cd-file = %q", q, cd)
		}
	}
}

func TestSwitchMovesMainAndCreatesWorktrees(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "work/later", "develop")
	testutil.Git(t, repo, "branch", "release/9.9", "develop")

	if got := strings.TrimSpace(Run(t, repo, "switch", "later").MustSucceed(t).Stdout); got != worktreeDir(repo, "work/later") || !exists(got) {
		t.Errorf("a branch without a worktree should get one, got %q", got)
	}
	res := Run(t, repo, "switch", "release/9.9").MustSucceed(t)
	if strings.TrimSpace(res.Stdout) != repo || current(t, repo) != "release/9.9" {
		t.Errorf("main should move onto the release; out=%q on %q", res.Stdout, current(t, repo))
	}
	Run(t, repo, "switch", "@").MustSucceed(t)
}

func TestSwitchRefusesDirtyMain(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "release/9.9", "develop")
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "dirty\n")
	res := Run(t, repo, "switch", "release/9.9").MustFail(t)
	if !strings.Contains(res.Stderr, "uncommitted") || current(t, repo) != "develop" {
		t.Errorf("stderr=%q on %q", res.Stderr, current(t, repo))
	}
}

func TestSwitchNeedsATerminalWhenAmbiguous(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "login").MustSucceed(t)
	Run(t, repo, "work", "start", "logout").MustSucceed(t)
	res := Run(t, repo, "switch", "log").MustFail(t)
	if !strings.Contains(res.Stderr, "no terminal") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}
