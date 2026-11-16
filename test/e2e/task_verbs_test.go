package e2e

import (
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func mustFailWith(t *testing.T, dir, want string, args ...string) {
	t.Helper()
	if res := Run(t, dir, args...).MustFail(t); !strings.Contains(res.Stderr, want) {
		t.Errorf("%v: stderr = %q, want %q", args, res.Stderr, want)
	}
}

func TestTaskVerbsResolveTheTask(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "login")
	fwt := worktreeDir(repo, "regular/login")
	commitFile(t, fwt, "a.txt")
	Run(t, fwt, "work", "start", "api").MustSucceed(t)
	api := worktreeDir(repo, "work/regular/login/api")

	for name, run := range map[string]func(args ...string) Result{
		"by id":              func(a ...string) Result { return Run(t, repo, append(a, "1")...) },
		"by #id":             func(a ...string) Result { return Run(t, repo, append(a, "#1")...) },
		"in the worktree":    func(a ...string) Result { return Run(t, fwt, a...) },
		"in a work worktree": func(a ...string) Result { return Run(t, api, a...) },
	} {
		if out := run("task", "diff").MustSucceed(t).Stdout; !strings.Contains(out, "a.txt") {
			t.Errorf("%s: diff = %q", name, out)
		}
		if out := run("task", "review").MustSucceed(t).Stdout; !strings.Contains(out, "'regular/login' is ready for review") {
			t.Errorf("%s: review = %q", name, out)
		}
		run("task", "review", "--undo").MustSucceed(t)
	}
	mustFailWith(t, repo, "not in a task's worktree; give the task number", "task", "review")
	mustFailWith(t, repo, "not in a task's worktree; give the task number", "task", "diff")
	mustFailWith(t, repo, "no task #9", "task", "review", "9")
	Run(t, repo, "task", "add", "Later").MustSucceed(t)
	mustFailWith(t, repo, "task #2 has no branch yet", "task", "diff", "2")
	mustFailWith(t, repo, "not a task number", "task", "diff", "regular/login")
}

func TestTaskCheckoutByIDMakesTheWorktree(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "login")
	wt := worktreeDir(repo, "regular/login")
	testutil.Git(t, repo, "worktree", "remove", "--force", wt)
	out := Run(t, repo, "task", "checkout", "1").MustSucceed(t).Stdout
	if !exists(wt) || !strings.Contains(out, wt) {
		t.Errorf("checkout printed %q, worktree exists: %v", out, exists(wt))
	}
	mustFailWith(t, repo, "accepts 1 arg", "task", "checkout")
}

func TestTaskReviewNeedsACard(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "regular/raw", "develop")
	wt := t.TempDir() + "/raw"
	testutil.Git(t, repo, "worktree", "add", wt, "regular/raw")
	mustFailWith(t, wt, "no task for 'regular/raw'; run `flowtree task sync`", "task", "review")
	if out := Run(t, repo, "task", "list").MustSucceed(t).Stdout; strings.Contains(out, "raw") {
		t.Errorf("review must not create a card:\n%s", out)
	}
}

func TestTaskFinishRegular(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "login")
	wt := worktreeDir(repo, "regular/login")
	commitFile(t, wt, "a.txt")
	mustFailWith(t, wt, "accepts 1 arg", "task", "finish")
	Run(t, repo, "task", "finish", "--dry-run", "1").MustSucceed(t)
	Run(t, repo, "task", "finish", "1").MustSucceed(t)
	if !exists(repo+"/a.txt") || exists(wt) || strings.Contains(branches(t, repo), "regular/login") {
		t.Errorf("not finished:\n%s", branches(t, repo))
	}
}

func TestTaskFinishRefusesFlagsOfTheOtherType(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "login")
	before := branches(t, repo)
	mustFailWith(t, repo, "--notag is for hotfix tasks", "task", "finish", "--notag", "1")
	mustFailWith(t, repo, "--message is for hotfix tasks", "task", "finish", "-m", "x", "1")
	if branches(t, repo) != before || !exists(worktreeDir(repo, "regular/login")) {
		t.Error("a refused finish changed something")
	}
}

func TestTaskFinishHotfixTagsAndBackMerges(t *testing.T) {
	repo := flowRepo(t)
	startTask(t, repo, "hotfix", "1.0.1")
	wt := worktreeDir(repo, "hotfix/1.0.1")
	commitFile(t, wt, "fix.txt")
	Run(t, repo, "task", "finish", "-m", "fix", "1").MustSucceed(t)
	if !strings.Contains(testutil.Git(t, repo, "tag"), "1.0.1") || !exists(repo+"/fix.txt") ||
		testutil.Git(t, repo, "branch", "--contains", "1.0.1", "--list", "develop") == "" {
		t.Error("hotfix should be tagged and back-merged into develop")
	}
	if exists(wt) || strings.Contains(branches(t, repo), "hotfix/1.0.1") {
		t.Error("hotfix left behind")
	}
}

func TestTaskFinishRefusesOpenWorkBranches(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "login")
	Run(t, repo, "work", "start", "-P", "1", "api").MustSucceed(t)
	mustFailWith(t, repo, "open work branches", "task", "finish", "1")
	mustFailWith(t, repo, "open work branches", "task", "delete", "-f", "1")
}

func TestTaskRebaseIsForRegularTasks(t *testing.T) {
	repo := flowRepo(t)
	startTask(t, repo, "hotfix", "1.0.1")
	mustFailWith(t, repo, "rebase is for regular tasks", "task", "rebase", "1")
	mustFailWith(t, repo, "pull is for regular tasks", "task", "pull", "1")

	startRegular(t, repo, "login") // task 2
	wt := worktreeDir(repo, "regular/login")
	commitFile(t, wt, "a.txt")
	commitFile(t, repo, "on-develop.txt")
	Run(t, wt, "task", "rebase").MustSucceed(t)
	if !exists(wt + "/on-develop.txt") {
		t.Error("rebase should bring develop's commit in")
	}
}

func TestTaskDelete(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "login")
	commitFile(t, worktreeDir(repo, "regular/login"), "a.txt")
	mustFailWith(t, repo, "has not been merged", "task", "delete", "1")
	Run(t, repo, "task", "delete", "-f", "1").MustSucceed(t)
	if strings.Contains(branches(t, repo), "regular/login") {
		t.Error("branch not deleted")
	}
}

func TestTaskPublishAndPull(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	startRegular(t, repo, "login")
	wt := worktreeDir(repo, "regular/login")
	commitFile(t, wt, "a.txt")
	Run(t, wt, "task", "publish").MustSucceed(t)
	if !strings.Contains(testutil.Git(t, remote, "branch"), "regular/login") {
		t.Error("not published")
	}
	Run(t, repo, "task", "pull", "1").MustSucceed(t)
}

func TestTaskTrackLinksAnExistingCard(t *testing.T) {
	repo, _ := flowRepoWithRemote(t)
	startRegular(t, repo, "login")
	wt := worktreeDir(repo, "regular/login")
	commitFile(t, wt, "a.txt")
	Run(t, wt, "task", "publish").MustSucceed(t)
	Run(t, repo, "task", "delete", "-f", "1").MustSucceed(t)
	Run(t, repo, "task", "add", "Again").MustSucceed(t)                      // 2, regular
	Run(t, repo, "task", "add", "--type", "hotfix", "Urgent").MustSucceed(t) // 3

	mustFailWith(t, repo, "is a hotfix task, not a regular", "task", "track", "3", "regular/login")
	mustFailWith(t, repo, "already belongs to", "task", "track", "1", "regular/login")
	mustFailWith(t, repo, "is not a regular or hotfix branch", "task", "track", "2", "develop")
	if strings.Contains(branches(t, repo), "regular/login") {
		t.Fatal("a refused track created the branch")
	}

	Run(t, repo, "task", "track", "2", "regular/login").MustSucceed(t)
	if !exists(wt) {
		t.Error("track should check the branch out")
	}
	if out := Run(t, repo, "task", "show", "2").MustSucceed(t).Stdout; !strings.Contains(out, "regular/login") {
		t.Errorf("task 2 not linked:\n%s", out)
	}
	if out := Run(t, repo, "task", "list").MustSucceed(t).Stdout; strings.Count(out, "#") != 3 {
		t.Errorf("track must not create a card:\n%s", out)
	}
}
