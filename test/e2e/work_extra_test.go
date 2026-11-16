package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestTaskDiffStartsAtTheMergeBase(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	commitFile(t, worktreeDir(repo, "regular/a"), "changed.txt")
	commitFile(t, repo, "on-develop.txt") // must not show up: the diff starts at the merge base

	out := Run(t, repo, "task", "diff", "1").MustSucceed(t).Stdout
	if !strings.Contains(out, "changed.txt") || strings.Contains(out, "on-develop.txt") {
		t.Errorf("diff:\n%s", out)
	}
	inferred := Run(t, worktreeDir(repo, "regular/a"), "task", "diff").MustSucceed(t).Stdout
	if inferred != out {
		t.Errorf("diff without a number should use the worktree's branch:\n%s", inferred)
	}
}

func TestTaskRebase(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	wt := worktreeDir(repo, "regular/a")
	commitFile(t, wt, "a.txt")
	commitFile(t, repo, "on-develop.txt")

	res := Run(t, repo, "task", "rebase", "1").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Will try to rebase 'a'") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if !exists(filepath.Join(wt, "on-develop.txt")) || !exists(filepath.Join(wt, "a.txt")) {
		t.Error("worktree should have both changes")
	}
	// linear on top of develop: develop is an ancestor and there are no merges
	testutil.Git(t, repo, "merge-base", "--is-ancestor", "develop", "regular/a")
	if out := testutil.Git(t, repo, "rev-list", "--merges", "develop..regular/a"); out != "" {
		t.Errorf("rebased branch has merges: %q", out)
	}
	if got := strings.Fields(testutil.Git(t, repo, "rev-list", "--parents", "-n1", "regular/a")); got[1] != strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "develop")) {
		t.Error("regular/a should sit directly on develop")
	}
}

func TestTaskRebaseConflictReportsAndLeavesRebaseInProgress(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	wt := worktreeDir(repo, "regular/a")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "work\n")
	testutil.Git(t, wt, "commit", "-q", "-am", "work edit")
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "develop\n")
	testutil.Git(t, repo, "commit", "-q", "-am", "develop edit")
	Run(t, repo, "task", "rebase", "1").MustFail(t)
}

func TestTaskPullFastForwards(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	startRegular(t, repo, "feat")
	wt := worktreeDir(repo, "regular/feat")
	Run(t, repo, "task", "publish", "1").MustSucceed(t)

	// someone else pushes a commit to regular/feat
	other := filepath.Join(filepath.Dir(repo), "other")
	testutil.Git(t, filepath.Dir(repo), "clone", "-q", remote, other)
	testutil.Git(t, other, "checkout", "-q", "regular/feat")
	commitFile(t, other, "theirs.txt")
	testutil.Git(t, other, "push", "-q", "origin", "regular/feat")

	Run(t, repo, "task", "pull", "1").MustSucceed(t)
	if !exists(filepath.Join(wt, "theirs.txt")) {
		t.Error("pull should fast-forward the worktree")
	}
}

func TestRelocate(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	startTask(t, repo, "hotfix", "1.0.1")
	oldA, oldH := worktreeDir(repo, "regular/a"), worktreeDir(repo, "hotfix/1.0.1")

	newBase := filepath.Join(filepath.Dir(repo), "moved")
	Run(t, repo, "config", "baseDir", newBase).MustSucceed(t)

	dry := Run(t, repo, "relocate", "-n").MustSucceed(t)
	if !strings.Contains(dry.Stdout, filepath.Join(newBase, "regular/a")) || !exists(oldA) || exists(filepath.Join(newBase, "regular/a")) {
		t.Errorf("dry run should only report:\n%s", dry.Stdout)
	}
	// refuses while standing inside a worktree that would move
	if res := Run(t, oldA, "relocate").MustFail(t); !strings.Contains(res.Stderr, "inside") {
		t.Errorf("stderr = %q", res.Stderr)
	}

	Run(t, repo, "relocate").MustSucceed(t)
	for _, b := range []string{"regular/a", "hotfix/1.0.1"} {
		if !exists(filepath.Join(newBase, filepath.FromSlash(b), "README.md")) {
			t.Errorf("%s was not moved under %s", b, newBase)
		}
	}
	if exists(oldA) || exists(oldH) || exists(filepath.Dir(oldA)) {
		t.Error("old locations (and their empty parents) should be gone")
	}
	if res := Run(t, repo, "relocate").MustSucceed(t); !strings.Contains(res.Stdout, "Nothing to relocate") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	// git still knows where they are, so commands keep working
	Run(t, repo, "task", "checkout", "1").MustSucceed(t)
}
