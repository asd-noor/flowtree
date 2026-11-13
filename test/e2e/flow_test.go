package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

// flowRepo is a repository set up with `init --flow` (production branch "main").
func flowRepo(t *testing.T) string {
	t.Helper()
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	return repo
}

// flowRepoWithRemote also has a bare origin carrying main and develop.
func flowRepoWithRemote(t *testing.T) (repo, remote string) {
	t.Helper()
	repo, remote = testutil.NewRepoWithRemote(t)
	Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	testutil.Git(t, repo, "push", "-q", "origin", "develop")
	return repo, remote
}

func worktreeDir(repo, branch string) string {
	return filepath.Join(filepath.Dir(repo), "worktrees", filepath.FromSlash(branch))
}

func commitFile(t *testing.T, dir, name string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, name), name+"\n")
	testutil.Git(t, dir, "add", "-A")
	testutil.Git(t, dir, "commit", "-q", "-m", "add "+name)
}

func branches(t *testing.T, repo string) string {
	return testutil.Git(t, repo, "branch", "--format=%(refname:short)")
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func current(t *testing.T, dir string) string {
	return strings.TrimSpace(testutil.Git(t, dir, "branch", "--show-current"))
}

// startTask adds a task of the given type titled name, starts its branch and returns the task number.
func startTask(t *testing.T, repo, typ, name string) (int, Result) {
	t.Helper()
	Run(t, repo, "task", "add", "--type", typ, name).MustSucceed(t)
	id := 0
	for n := range tasks(t, repo) {
		id = max(id, n)
	}
	return id, Run(t, repo, "task", "start", strconv.Itoa(id), name).MustSucceed(t)
}

func startRegular(t *testing.T, repo, name string) int {
	t.Helper()
	id, _ := startTask(t, repo, "regular", name)
	return id
}

func TestRegularLifecycle(t *testing.T) {
	repo := flowRepo(t)
	_, res := startTask(t, repo, "regular", "login")
	wt := worktreeDir(repo, "regular/login")
	if !exists(wt) || !strings.Contains(res.Stdout, wt) {
		t.Fatalf("worktree not created at %s:\n%s", wt, res.Stdout)
	}
	if got := Run(t, repo, "task", "list").MustSucceed(t).Stdout; !strings.Contains(got, "login") {
		t.Errorf("list: %q", got)
	}
	commitFile(t, wt, "login.txt")

	cdfile := filepath.Join(t.TempDir(), "cd")
	res = RunEnv(t, wt, []string{"FLOWTREE_CDFILE=" + cdfile}, "task", "finish", "1").MustSucceed(t)
	if exists(wt) || strings.Contains(branches(t, repo), "regular/login") {
		t.Errorf("worktree or branch left behind:\n%s\n%s", res.Stdout, branches(t, repo))
	}
	if !exists(filepath.Join(repo, "login.txt")) || current(t, repo) != "develop" {
		t.Error("work was not merged into develop in the main worktree")
	}
	if cd, _ := os.ReadFile(cdfile); strings.TrimSpace(string(cd)) != repo {
		t.Errorf("shell should be sent to the main worktree, got %q", cd)
	}
}

func TestRegularFinishAlwaysCreatesAMergeCommit(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	commitFile(t, worktreeDir(repo, "regular/a"), "a1") // develop did not move: a fast-forward would be possible
	Run(t, repo, "task", "finish", "1").MustSucceed(t)
	if got := len(strings.Fields(testutil.Git(t, repo, "rev-list", "--parents", "-n1", "develop"))) - 1; got != 2 {
		t.Errorf("develop head has %d parents, want a merge commit", got)
	}
	for _, flag := range []string{"--no-ff", "-r", "--no-rebase"} {
		if res := Run(t, repo, "task", "finish", "1", flag).MustFail(t); !strings.Contains(res.Stderr, "unknown") {
			t.Errorf("%s: stderr = %q", flag, res.Stderr)
		}
	}
}

func TestRegularFinishAfterPullRequestOnlyCleansUp(t *testing.T) {
	repo, _ := flowRepoWithRemote(t)
	startRegular(t, repo, "a")
	commitFile(t, worktreeDir(repo, "regular/a"), "a")
	Run(t, repo, "task", "publish", "1").MustSucceed(t)
	testutil.Git(t, repo, "push", "-q", "origin", "regular/a:develop") // the forge merged the pull request
	before := testutil.Git(t, repo, "rev-parse", "develop")

	res := Run(t, repo, "task", "finish", "1").MustSucceed(t)
	if !strings.Contains(res.Stdout, "already merged") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if testutil.Git(t, repo, "rev-parse", "develop") != before {
		t.Error("finish must not merge again when the pull request already did")
	}
	if strings.Contains(branches(t, repo), "regular/a") || exists(worktreeDir(repo, "regular/a")) {
		t.Error("the branch and its worktree should be gone")
	}
}

func TestStartRefusals(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "x")
	Run(t, repo, "task", "add", "Again").MustSucceed(t)
	Run(t, repo, "task", "add", "--type", "hotfix", "Crash").MustSucceed(t)
	if res := Run(t, repo, "task", "start", "2", "x").MustFail(t); !strings.Contains(res.Stderr, "already exists") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	for _, args := range [][]string{{"release", "start", "1.1.0"}, {"task", "start", "3", "1.0.1"}} {
		if res := Run(t, repo, args...).MustFail(t); !strings.Contains(res.Stderr, "finish that one first") {
			t.Errorf("%v: stderr = %q", args, res.Stderr)
		}
	}
	if res := Run(t, repo, "task", "start", "2").MustFail(t); res.Stderr == "" {
		t.Error("missing name should be reported")
	}
}

func TestFlowNeedsInit(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-d").MustSucceed(t)
	if res := Run(t, repo, "release", "list").MustFail(t); !strings.Contains(res.Stderr, "init --flow") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestReleaseLifecycle(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	wt := worktreeDir(repo, "release/1.0.0")
	if current(t, repo) != "develop" || !exists(wt) {
		t.Fatalf("the release should get a worktree, main stays on develop (is on %q)", current(t, repo))
	}
	commitFile(t, wt, "CHANGELOG.md")
	Run(t, wt, "release", "finish", "-m", "Release 1.0.0").MustSucceed(t)

	if exists(wt) || strings.Contains(branches(t, repo), "release/1.0.0") {
		t.Errorf("the release worktree and branch should be gone\n%s", branches(t, repo))
	}
	if got := testutil.Git(t, repo, "tag", "-l", "1.0.0"); !strings.Contains(got, "1.0.0") {
		t.Fatal("tag missing")
	}
	if got := testutil.Git(t, repo, "cat-file", "-t", "1.0.0"); strings.TrimSpace(got) != "tag" {
		t.Errorf("tag should be annotated, is %q", got)
	}
	for _, b := range []string{"main", "develop"} {
		if out := testutil.Git(t, repo, "ls-tree", "--name-only", b); !strings.Contains(out, "CHANGELOG.md") {
			t.Errorf("%s lacks the release changes", b)
		}
	}
	Run(t, repo, "release", "list").MustSucceed(t) // no release left; just must not fail
}

func TestReleaseFinishFlags(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "2.0.0").MustSucceed(t)
	wt := worktreeDir(repo, "release/2.0.0")
	commitFile(t, wt, "x")
	Run(t, wt, "release", "finish", "-n", "-b", "-k").MustSucceed(t) // no tag, no back-merge, keep branch
	if testutil.Git(t, repo, "tag", "-l") != "" {
		t.Error("-n must not tag")
	}
	if !strings.Contains(branches(t, repo), "release/2.0.0") {
		t.Error("-k must keep the branch")
	}
	if exists(wt) {
		t.Error("-k keeps the branch, not the worktree")
	}
	if strings.Contains(testutil.Git(t, repo, "ls-tree", "--name-only", "develop"), "x") {
		t.Error("-b must not merge into develop")
	}
}

func TestReleaseTagIsTheNameAsTyped(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "v2.0.0").MustSucceed(t)
	commitFile(t, worktreeDir(repo, "release/v2.0.0"), "x")
	Run(t, repo, "release", "finish", "-m", "m", "v2.0.0").MustSucceed(t)
	if got := strings.TrimSpace(testutil.Git(t, repo, "tag", "-l")); got != "v2.0.0" {
		t.Errorf("tag = %q, want the release name", got)
	}
}

func TestHotfixLifecycleWithPush(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	startTask(t, repo, "hotfix", "1.0.1")
	wt := worktreeDir(repo, "hotfix/1.0.1")
	if !exists(wt) {
		t.Fatal("hotfix worktree missing")
	}
	commitFile(t, wt, "fix.txt")
	Run(t, wt, "task", "finish", "-p", "-m", "fix", "1").MustSucceed(t)

	if exists(wt) || strings.Contains(branches(t, repo), "hotfix/1.0.1") {
		t.Error("hotfix worktree or branch left behind")
	}
	for _, b := range []string{"main", "develop"} {
		if !strings.Contains(testutil.Git(t, remote, "ls-tree", "--name-only", b), "fix.txt") {
			t.Errorf("remote %s lacks the hotfix", b)
		}
	}
	if !strings.Contains(testutil.Git(t, remote, "tag", "-l"), "1.0.1") {
		t.Error("tag was not pushed")
	}
}

func TestFinishConflictChangesNothing(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	wt := worktreeDir(repo, "regular/a")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "from work\n")
	testutil.Git(t, wt, "commit", "-q", "-am", "work edit")
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "from develop\n")
	testutil.Git(t, repo, "commit", "-q", "-am", "develop edit")
	before := testutil.Git(t, repo, "rev-parse", "develop")

	res := Run(t, repo, "task", "finish", "1").MustFail(t)
	if !strings.Contains(res.Stderr, "Nothing was changed") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if testutil.Git(t, repo, "rev-parse", "develop") != before || testutil.Git(t, repo, "status", "--porcelain", "--untracked-files=no") != "" || !exists(wt) {
		t.Error("state changed after a failed finish")
	}
}

func TestFinishRefusesDirtyWorktree(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	wt := worktreeDir(repo, "regular/a")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "dirty\n")
	if res := Run(t, repo, "task", "finish", "1").MustFail(t); !strings.Contains(res.Stderr, "uncommitted") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "a").MustSucceed(t)
	res := Run(t, repo, "task", "start", "1", "a", "--dry-run").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Dry run") || strings.Contains(branches(t, repo), "regular/a") || exists(worktreeDir(repo, "regular/a")) {
		t.Errorf("start --dry-run changed things or printed nothing:\n%s", res.Stdout)
	}
	startRegular(t, repo, "b") // task 2
	commitFile(t, worktreeDir(repo, "regular/b"), "b")
	Run(t, repo, "task", "finish", "2", "--dry-run").MustSucceed(t)
	if !exists(worktreeDir(repo, "regular/b")) || exists(filepath.Join(repo, "b")) {
		t.Error("finish --dry-run changed things")
	}
	Run(t, repo, "task", "delete", "2", "-f", "--dry-run").MustSucceed(t)
	if !strings.Contains(branches(t, repo), "regular/b") {
		t.Error("delete --dry-run changed things")
	}
}

func TestDelete(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	wt := worktreeDir(repo, "regular/a")
	commitFile(t, wt, "a")
	if res := Run(t, repo, "task", "delete", "1").MustFail(t); !strings.Contains(res.Stderr, "not been merged") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	Run(t, repo, "task", "delete", "-f", "1").MustSucceed(t)
	if exists(wt) || strings.Contains(branches(t, repo), "regular/a") {
		t.Error("delete -f should remove worktree and branch")
	}
}

func TestPublishTrackCheckout(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	startRegular(t, repo, "feat")
	Run(t, repo, "task", "publish", "1").MustSucceed(t)
	if !strings.Contains(testutil.Git(t, remote, "branch", "--list"), "regular/feat") {
		t.Fatal("publish did not push")
	}
	Run(t, repo, "task", "finish", "1", "-k", "--keepremote").MustSucceed(t) // branch kept, worktree removed
	wt := worktreeDir(repo, "regular/feat")
	if exists(wt) {
		t.Fatal("finish -k should still remove the worktree")
	}
	res := Run(t, repo, "task", "checkout", "1").MustSucceed(t) // recreates it
	if strings.TrimSpace(res.Stdout) != wt || !exists(wt) {
		t.Errorf("checkout printed %q, want %s", res.Stdout, wt)
	}
	Run(t, repo, "task", "finish", "1").MustSucceed(t)
	if strings.Contains(testutil.Git(t, remote, "branch", "--list"), "regular/feat") {
		t.Error("finish should delete the remote branch")
	}

	testutil.Git(t, repo, "push", "-q", "origin", "develop:regular/other")
	Run(t, repo, "task", "add", "other").MustSucceed(t)
	Run(t, repo, "task", "track", "2", "regular/other").MustSucceed(t)
	if !exists(worktreeDir(repo, "regular/other")) {
		t.Error("track should create a worktree")
	}
}

func TestReleaseCheckoutRecreatesWorktree(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	wt := worktreeDir(repo, "release/1.0.0")
	testutil.Git(t, repo, "worktree", "remove", wt)
	res := Run(t, repo, "release", "checkout").MustFail(t) // not in a release worktree, no name
	if res.Stderr == "" {
		t.Error("expected an error")
	}
	Run(t, repo, "release", "checkout", "1.0.0").MustSucceed(t)
	if !exists(wt) || current(t, repo) != "develop" {
		t.Errorf("checkout should recreate the worktree; main on %q", current(t, repo))
	}
}

func TestReleaseFinishResumesAfterBackMergeConflict(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	wt := worktreeDir(repo, "release/1.0.0")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "release notes\n")
	testutil.Git(t, wt, "commit", "-q", "-am", "release edit")
	// develop (in the main worktree) moves on with a conflicting edit while the release is open
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "develop notes\n")
	testutil.Git(t, repo, "commit", "-q", "-am", "develop edit")

	res := Run(t, wt, "release", "finish", "-m", "1.0.0").MustFail(t)
	if !strings.Contains(res.Stderr, "by hand") || !strings.Contains(testutil.Git(t, repo, "tag", "-l"), "1.0.0") {
		t.Fatalf("expected a back-merge failure after tagging; stderr = %q", res.Stderr)
	}

	// resolve by hand, as the error says: merge the tag into develop
	testutil.Git(t, repo, "merge", "-q", "-X", "ours", "--no-edit", "1.0.0")

	res = Run(t, wt, "release", "finish", "-m", "1.0.0").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Resuming") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if exists(wt) || strings.Contains(branches(t, repo), "release/1.0.0") {
		t.Errorf("finish should complete\n%s", branches(t, repo))
	}
}

func TestFlowCompletion(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "Login").MustSucceed(t)
	Run(t, repo, "task", "add", "Signup").MustSucceed(t)
	Run(t, repo, "task", "start", "1", "login").MustSucceed(t)
	out := Run(t, repo, "__complete", "task", "start", "").MustSucceed(t).Stdout
	if !strings.Contains(out, "2\tSignup") || strings.Contains(out, "Login") {
		t.Errorf("task start completion offers tasks without a branch: %q", out)
	}
	out = Run(t, repo, "__complete", "task", "").MustSucceed(t).Stdout
	if !strings.Contains(out, "start") {
		t.Errorf("subcommand completion: %q", out)
	}
}
