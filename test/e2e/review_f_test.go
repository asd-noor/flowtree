package e2e

import (
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

// A branch with no commits of its own is not an interrupted finish: the message must not claim a resume.
func TestFinishOfAnEmptyHotfixDoesNotClaimToResume(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "hotfix", "start", "1.0.1").MustSucceed(t)
	res := Run(t, repo, "hotfix", "finish", "1.0.1", "-m", "m").MustSucceed(t)
	if strings.Contains(res.Stdout, "Resuming") || !strings.Contains(res.Stdout, "no commits of its own") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if !strings.Contains(testutil.Git(t, repo, "tag", "-l"), "1.0.1") {
		t.Error("the unchanged master should still be tagged")
	}
}

func TestFinishOfAnEmptyReleaseWithoutTagSaysNothingToMerge(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "2.0.0").MustSucceed(t)
	res := Run(t, repo, "release", "finish", "2.0.0", "-n", "-b").MustSucceed(t)
	if strings.Contains(res.Stdout, "Resuming") || !strings.Contains(res.Stdout, "no commits of its own") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if current(t, repo) != "develop" {
		t.Errorf("main worktree on %q", current(t, repo))
	}
}
