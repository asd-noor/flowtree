package e2e

import (
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestTaskSyncFollowsRawGit(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "regular/raw", "develop")
	testutil.Git(t, repo, "branch", "work/regular/raw/sub", "regular/raw")

	if out := Run(t, repo, "task", "sync", "-n").MustSucceed(t).Stdout; !strings.Contains(out, "Dry run") || len(tasks(t, repo)) != 0 {
		t.Fatalf("-n must not write:\n%s", out)
	}
	out := Run(t, repo, "task", "sync").MustSucceed(t).Stdout
	if !strings.Contains(out, "added task #1 for regular/raw") || !strings.Contains(out, "added subtask sub to task #1") {
		t.Errorf("sync:\n%s", out)
	}
	if v := tasks(t, repo)[1]; v.Status != "doing" || len(v.Subtasks) != 1 {
		t.Errorf("after sync: %+v", v)
	}

	// commit on the branch, merge it by hand, delete it: the recorded tip still says done
	testutil.Git(t, repo, "checkout", "-q", "regular/raw")
	commitFile(t, repo, "raw.txt")
	Run(t, repo, "task", "sync").MustSucceed(t)
	testutil.Git(t, repo, "branch", "-q", "-m", "regular/raw", "regular/renamed")
	if out := Run(t, repo, "task", "sync").MustSucceed(t).Stdout; !strings.Contains(out, "relinked task #1 from regular/raw to regular/renamed") {
		t.Errorf("rename:\n%s", out)
	}
	testutil.Git(t, repo, "checkout", "-q", "develop")
	testutil.Git(t, repo, "merge", "-q", "--no-ff", "--no-edit", "regular/renamed")
	testutil.Git(t, repo, "branch", "-q", "-D", "regular/renamed", "work/regular/raw/sub")
	if v := tasks(t, repo)[1]; v.Status != "done" {
		t.Errorf("merged and deleted by hand: %s", v.Status)
	}
	if out := Run(t, repo, "task", "sync").MustSucceed(t).Stdout; !strings.Contains(out, "in line") {
		t.Errorf("nothing left to do:\n%s", out)
	}
}
