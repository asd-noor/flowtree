package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestWorkDiff(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	commitFile(t, worktreeDir(repo, "work/a"), "changed.txt")
	commitFile(t, repo, "on-develop.txt") // must not show up: the diff starts at the merge base

	out := Run(t, repo, "work", "diff", "a").MustSucceed(t).Stdout
	if !strings.Contains(out, "changed.txt") || strings.Contains(out, "on-develop.txt") {
		t.Errorf("diff:\n%s", out)
	}
	inferred := Run(t, worktreeDir(repo, "work/a"), "work", "diff").MustSucceed(t).Stdout
	if inferred != out {
		t.Errorf("diff without a name should use the worktree's branch:\n%s", inferred)
	}
}

func TestWorkRebase(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "work/a")
	commitFile(t, wt, "a.txt")
	commitFile(t, repo, "on-develop.txt")

	res := Run(t, repo, "work", "rebase", "a").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Will try to rebase 'a'") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if !exists(filepath.Join(wt, "on-develop.txt")) || !exists(filepath.Join(wt, "a.txt")) {
		t.Error("worktree should have both changes")
	}
	// linear on top of develop: develop is an ancestor and there are no merges
	testutil.Git(t, repo, "merge-base", "--is-ancestor", "develop", "work/a")
	if out := testutil.Git(t, repo, "rev-list", "--merges", "develop..work/a"); out != "" {
		t.Errorf("rebased branch has merges: %q", out)
	}
	if got := strings.Fields(testutil.Git(t, repo, "rev-list", "--parents", "-n1", "work/a")); got[1] != strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "develop")) {
		t.Error("work/a should sit directly on develop")
	}
}

func TestWorkRebaseConflictReportsAndLeavesRebaseInProgress(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "work/a")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "work\n")
	testutil.Git(t, wt, "commit", "-q", "-am", "work edit")
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "develop\n")
	testutil.Git(t, repo, "commit", "-q", "-am", "develop edit")
	Run(t, repo, "work", "rebase", "a").MustFail(t)
}

func TestWorkPullFastForwardsAndCreates(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	Run(t, repo, "work", "start", "feat").MustSucceed(t)
	wt := worktreeDir(repo, "work/feat")
	Run(t, repo, "work", "publish", "feat").MustSucceed(t)

	// someone else pushes a commit to work/feat
	other := filepath.Join(filepath.Dir(repo), "other")
	testutil.Git(t, filepath.Dir(repo), "clone", "-q", remote, other)
	testutil.Git(t, other, "checkout", "-q", "work/feat")
	commitFile(t, other, "theirs.txt")
	testutil.Git(t, other, "push", "-q", "origin", "work/feat")

	Run(t, repo, "work", "pull", "origin", "feat").MustSucceed(t)
	if !exists(filepath.Join(wt, "theirs.txt")) {
		t.Error("pull should fast-forward the worktree")
	}

	// a branch that only exists on the remote is created, with a worktree
	testutil.Git(t, other, "checkout", "-q", "-b", "work/new", "origin/develop")
	commitFile(t, other, "new.txt")
	testutil.Git(t, other, "push", "-q", "origin", "work/new")
	cdfile := filepath.Join(t.TempDir(), "cd")
	RunEnv(t, repo, []string{"FLOWTREE_CDFILE=" + cdfile}, "work", "pull", "origin", "new").MustSucceed(t)
	newWt := worktreeDir(repo, "work/new")
	if !exists(filepath.Join(newWt, "new.txt")) {
		t.Fatal("pull should create the missing branch and its worktree")
	}
	if cd, _ := os.ReadFile(cdfile); strings.TrimSpace(string(cd)) != newWt {
		t.Errorf("cd file = %q, want %s", cd, newWt)
	}
	if res := Run(t, repo, "work", "pull", "origin", "nope").MustFail(t); !strings.Contains(res.Stderr, "not found") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestRelocate(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	Run(t, repo, "hotfix", "start", "1.0.1").MustSucceed(t)
	oldA, oldH := worktreeDir(repo, "work/a"), worktreeDir(repo, "hotfix/1.0.1")

	newBase := filepath.Join(filepath.Dir(repo), "moved")
	cfg, _ := os.ReadFile(filepath.Join(repo, ".flowtree.json"))
	edited := strings.Replace(string(cfg), `"base_dir": "../worktrees"`, `"base_dir": "`+newBase+`"`, 1)
	if edited == string(cfg) {
		t.Fatalf("could not edit base_dir in:\n%s", cfg)
	}
	testutil.WriteFile(t, filepath.Join(repo, ".flowtree.json"), edited)

	dry := Run(t, repo, "relocate", "-n").MustSucceed(t)
	if !strings.Contains(dry.Stdout, filepath.Join(newBase, "work/a")) || !exists(oldA) || exists(filepath.Join(newBase, "work/a")) {
		t.Errorf("dry run should only report:\n%s", dry.Stdout)
	}
	// refuses while standing inside a worktree that would move
	if res := Run(t, oldA, "relocate").MustFail(t); !strings.Contains(res.Stderr, "inside") {
		t.Errorf("stderr = %q", res.Stderr)
	}

	Run(t, repo, "relocate").MustSucceed(t)
	for _, b := range []string{"work/a", "hotfix/1.0.1"} {
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
	Run(t, repo, "work", "checkout", "a").MustSucceed(t)
}
