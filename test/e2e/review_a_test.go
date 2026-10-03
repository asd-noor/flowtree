package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

// checkedOutByHand puts branch into the main worktree and develop into a second worktree.
func checkedOutByHand(t *testing.T, repo, branch, base string) string {
	t.Helper()
	testutil.Git(t, repo, "checkout", "-q", "-b", branch, base)
	commitFile(t, repo, "by-hand.txt")
	other := filepath.Join(filepath.Dir(repo), "develop-elsewhere")
	testutil.Git(t, repo, "worktree", "add", "-q", other, "develop")
	return other
}

func TestFinishRefusesWhenMainCannotReturnToDevelop(t *testing.T) {
	for _, tc := range []struct{ kind, name, branch, base string }{
		{"work", "x", "work/x", "develop"},
		{"hotfix", "1.0.1", "hotfix/1.0.1", "main"},
	} {
		repo := flowRepo(t)
		other := checkedOutByHand(t, repo, tc.branch, tc.base)
		before := testutil.Git(t, repo, "rev-parse", "develop")

		args := []string{tc.kind, "finish", tc.name}
		if tc.kind != "work" {
			args = append(args, "-m", "m")
		}
		res := Run(t, repo, args...).MustFail(t)
		if !strings.Contains(res.Stderr, "cannot return") || !strings.Contains(res.Stderr, other) {
			t.Errorf("%s: stderr = %q", tc.kind, res.Stderr)
		}
		if testutil.Git(t, repo, "rev-parse", "develop") != before || current(t, repo) != tc.branch || !strings.Contains(branches(t, repo), tc.branch) {
			t.Errorf("%s: a refused finish must change nothing", tc.kind)
		}
	}
}

// A signing failure happens after master was merged. The error says so, and the same command run again
// (without the failing option) completes. Skipped where signing happens to work.
func TestFailedTagReportsAndRerunCompletes(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	commitFile(t, repo, "CHANGELOG.md")

	res := Run(t, repo, "release", "finish", "-s", "-m", "1.0.0")
	if res.Code == 0 {
		t.Skip("tag signing works in this environment")
	}
	if !strings.Contains(res.Stderr, "already merged into 'main'") || !strings.Contains(res.Stderr, "re-run 'flowtree release finish 1.0.0'") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
	if !strings.Contains(testutil.Git(t, repo, "ls-tree", "--name-only", "main"), "CHANGELOG.md") {
		t.Fatal("master should already contain the release")
	}

	out := Run(t, repo, "release", "finish", "-m", "1.0.0").MustSucceed(t).Stdout
	if !strings.Contains(out, "Resuming") || !strings.Contains(testutil.Git(t, repo, "tag", "-l"), "1.0.0") {
		t.Errorf("re-run should resume and tag:\n%s", out)
	}
	if current(t, repo) != "develop" || strings.Contains(branches(t, repo), "release/1.0.0") {
		t.Errorf("finish should complete: on %q\n%s", current(t, repo), branches(t, repo))
	}
}
